# nns   

## Name

*nns* - enables serving data from [neo blockchain](https://neo.org/).

## Description

The nns plugin try to get value from TXT records in provided neo node 
(lookup in [NNS smart contract](https://docs.neo.org/docs/en-us/reference/nns.html)).

## Syntax

``` txt
nns MORPH_CHAIN_ENDPOINT
```

## Examples

In this configuration, we try to first find the result in the provided neo node and forward 
requests to 8.8.8.8 if the neo request fails.

``` corefile
. {
  nns http://localhost:30333
  forward . 8.8.8.8
}
```
