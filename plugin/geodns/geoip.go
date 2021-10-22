package geodns

import (
	"fmt"
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

type db struct {
	readers map[int]*geoip2.Reader
	m       sync.RWMutex
}

const (
	isCity = 1 << iota
	isCountry
)

var probingIP = net.ParseIP("127.0.0.1")

func typeToString(dbType int) string {
	switch dbType {
	case isCity:
		return "city"
	case isCountry:
		return "country"
	}

	return fmt.Sprintf("unkonwn type %d", dbType)
}

func (db *db) AddReader(dbType int, r *geoip2.Reader) {
	db.m.Lock()
	db.readers[dbType] = r
	db.m.Unlock()
}

func (db *db) Reader(dbType int) (*geoip2.Reader, error) {
	db.m.RLock()
	defer db.m.RUnlock()

	r, ok := db.readers[dbType]
	if !ok {
		return nil, fmt.Errorf("db with type %d not found", dbType)
	}
	return r, nil
}

type IPInformation struct {
	City    *geoip2.City
	Country *geoip2.Country
}

type DistanceInfo struct {
	Distance       float64
	CountryMatched bool
}

func (i *IPInformation) IsEmpty() bool {
	if i.City == nil && i.Country == nil {
		return true
	}

	if i.City != nil && i.City.Location == emptyLocation.Location && i.Country == nil {
		return true
	}

	return false
}

func (db *db) IPInfo(ip net.IP) *IPInformation {
	result := &IPInformation{}

	cityDB, err := db.Reader(isCity)
	if err == nil {
		city, err := cityDB.City(ip)
		if err != nil {
			log.Debugf("couldn't get data from city db: %s", err.Error())
		} else {
			result.City = city

		}
	}

	countryDB, err := db.Reader(isCountry)
	if err == nil {
		country, err := countryDB.Country(ip)
		if err != nil {
			log.Debugf("couldn't get data from country db: %s", err.Error())
		} else {
			result.Country = country
		}
	}

	return result
}

func getDBType(r *geoip2.Reader) (int, error) {
	if _, err := r.City(probingIP); err != nil {
		if _, ok := err.(geoip2.InvalidMethodError); !ok {
			return 0, fmt.Errorf("couldn't look up database %s: %w", r.Metadata().DatabaseType, err)
		}
	} else {
		return isCity, nil
	}

	if _, err := r.Country(probingIP); err != nil {
		if _, ok := err.(geoip2.InvalidMethodError); !ok {
			return 0, fmt.Errorf("couldn't look up database %s: %w", r.Metadata().DatabaseType, err)
		}
	} else {
		return isCountry, nil
	}

	return 0, fmt.Errorf("unkonwn db type: %s", r.Metadata().DatabaseType)
}
