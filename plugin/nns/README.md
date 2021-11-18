# nns   

## Name

*nns* - enables serving data from [neo blockchain](https://neo.org/).

## Description

The nns plugin try to get value from TXT records in provided neo node 
(lookup in [NNS smart contract](https://docs.neo.org/docs/en-us/reference/nns.html)).
You can specify NNS domain to map DNS domain from request (default no mapping).

## Syntax

``` txt
nns NEO_N3_CHAIN_ENDPOINT [NNS_DOMAIN]
```

## Examples

In this configuration, first we try to find the result in the provided neo node and forward 
requests to 8.8.8.8 if the neo request fails.

``` corefile
. {
  nns http://localhost:30333
  forward . 8.8.8.8
}
```

This example shows how to map `containers.testnet.fs.neo.org` dns domain to `containers` nns domain 
(so request for `nicename.containers.testnet.fs.neo.org` will transform to `nicename.containers`).
It also enables zone transfer support:

``` corefile
containers.testnet.fs.neo.org {
  nns http://morph_chain.neofs.devenv:30333 containers
  transfer {
      to *
  }
}
```

If there is no domain filter in config:

``` corefile
. {
  nns http://morph_chain.neofs.devenv:30333 containers
}
```

Request for `nicename.containers.testnet.fs.neo.org` will transform to `nicename.containers.testnet.fs.neo.org.containers`.
