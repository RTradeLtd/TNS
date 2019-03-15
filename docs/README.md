# TNS (temporal name server)

TNS is an experimental solution at scaling IPNS name resolution, by mimicking some of the concepts in DNS.

It currently uses [dnsaddr](https://github.com/multiformats/go-multiaddr-dns) to retrieve the multiaddrs needed to talk to other TNS daemons.

## Glossary

* Authoritative Request
  * Implies receiving an answer by talking directly with the publisher.
  * we connect directly to the TNS node that the publisher of a record is running.
* Recursive Request
  * A recursive request is a request where we ask multiple people for the answer
  * this means we query the IPNS network directly

## Overview

There are two types of requests, authoritative and recursive. When making an authoritative we lookup the answer directly from the node who published it. When making recursive requests we lookup the response directly from the IPNS/IPFS network.