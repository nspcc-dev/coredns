# geodns

## Name

*geodns* - Lookup maxmind geoip2 databases using the response servers IP and filter by distance to the client IP.

## Description

The geodns plugin filter response dns records (types: `A, AAAA`) and transfer only closest to the client. 
You can specify max allowed records to response (default is 1).

## Syntax

``` txt
geodns GEOIP_DATABASE [MAX_RECORDS]
```

## Examples

In this configuration, we will filter `A` and `AAAA` records that nns plugin found in the NEO blockchain.

``` corefile
. {
   geodns testdata/GeoIP2-City-Test.mmdb 2
   nns http://localhost:30333
}
```
