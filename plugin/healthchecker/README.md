# healthchecker

## Name

*healthchecker* - filters records with unhealthy IPs (types: `A, AAAA`).

## Description

A healthchecker plugin filters input DNS records and returns healthy records. To response fast, it stores records and 
their statuses in LRU cache and responses in the following way:
1. if the record is not found in the cache the plugin returns the records as healthy, triggers check and puts it into 
the cache
2. if the record is found in the cache the plugin returns the record if it's healthy

Also, the plugin can be configured, what record names will be checked. If name filters are set, the plugin will check  
and store in cache only records which suite with the filters, otherwise the record will always be returned 
as healthy. If the filter is not set, the plugin will check and store all records.

## Syntax

``` txt
healthchecker HEALTHCHECK_METHOD CACHE_SIZE HEALTHCHECK_INTERVAL ORIGIN. [NAME_FILTER1 NAME_FILTER2 ...]
```

- `HEALTHCHECK_METHOD` -- method of checking of nodes: http is implemented.  
  
HTTP method can be configured in the following format: `http OR http:PORT OR http:PORT:TIMEOUT_IN_MS`

- `CACHE_SIZE` -- maximum number of records in cache
- `HEALTHCHECK_INTERVAL` -- time interval of updating status of records in cache in duration format
- `ORIGIN.` -- DNS origin. Must end with a dot.
- `[NAME_FILTER1 NAME_FILTER2 ...]` -- optional filters for names of records. It can be an exact name or
  '@' (means the same name as `ORIGIN`).

## Examples

In this configuration, we will filter `A` and `AAAA` records, store maximum 1000 records in cache, and start recheck of 
each record in cache for every 3 seconds via http client. The plugin will check records with name 
fs.neo.org ('@' in config) or cdn.fs.neo.org ('cdn' in config).
HTTP requests to check and update statuses of IPs will use default 80 port and wait for default 2 seconds.
``` corefile
. {
    healthchecker http 1000 1s fs.neo.org. @ cdn
    file db.example.org fs.neo.org
}
```

The same as above but port and timeout for HTTP client are set.
``` corefile
. {
    healthchecker http:80:3000 1000 1s fs.neo.org. @ cdn
    file db.example.org fs.neo.org
}
```