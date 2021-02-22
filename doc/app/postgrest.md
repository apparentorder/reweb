# Overview

Seems to run fine at first glance. (Only unauth'd GET tested)

- Needs VPC for PostgreSQL connectivity (unless using public database IP), but nothing else
- Seems to be fine with <100 MB RAM
- Amazingly fast!

# Environment

| Env | Value |
| --- | --- |
| PGRST_DB_URI | postgres://postgres:sUperSecr3t@pg-cluster.cluster-foobar.eu-central-1.rds.amazonaws.com:5432/dbname |
| PGRST_DB_SCHEMA | public |
| PGRST_DB_ANON_ROLE | postgres |

# Dockerfile
```
FROM public.ecr.aws/apparentorder/reweb as reweb

FROM postgrest/postgrest
COPY --from=reweb /reweb /reweb

ENV REWEB_APPLICATION_EXEC postgrest /etc/postgrest.conf
ENV REWEB_APPLICATION_PORT 3000
ENV REWEB_WAIT_CODE 200

ENTRYPOINT ["/reweb"]
```

#### Rationale

- Wait Code is necessary because PostgREST will serve HTTP/503 until the database connection has been established
