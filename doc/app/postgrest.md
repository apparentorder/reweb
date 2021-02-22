## Overview

Seems to run fine at first glance. (Only unauth'd GET tested)

- Needs VPC for PostgreSQL connectivity (unless using public database IP), but nothing else
- Seems to be fine with <100 MB RAM
- Amazingly fast!

## Environment

| Env | Value |
| --- | --- |
| PGRST_DB_URI | postgres://postgres:sUperSecr3t@pg-cluster.cluster-foobar.eu-central-1.rds.amazonaws.com:5432/dbname |
| PGRST_DB_SCHEMA | public |
| PGRST_DB_ANON_ROLE | postgres |

For optional config values, see the container docs below.

## Dockerfile
```
FROM public.ecr.aws/apparentorder/reweb as reweb

FROM postgrest/postgrest
COPY --from=reweb /reweb /reweb

ENV PGRST_DB_POOL 1

ENV REWEB_APPLICATION_EXEC postgrest /etc/postgrest.conf
ENV REWEB_APPLICATION_PORT 3000
ENV REWEB_WAIT_CODE 200

ENTRYPOINT ["/reweb"]
```

#### Rationale

- Any single PostgREST Lambda instance will never service more than one request concurrently, therefore the default PGRST_DB_POOL (100) can be reduced to 1
- Wait Code is necessary because PostgREST will serve HTTP/503 until the database connection has been established

## References

- Container overview and config: https://hub.docker.com/r/postgrest/postgrest
