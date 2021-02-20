# Overview

Works great for viewing data!

- Needs some PostgreSQL for sessions and config (Aurora Serverless works)
- Might need internet connectivity (NAT) for data sources (e.g. endpoints for Timestream, Cloudwatch, ... unless private endpoints)

⚠️ Any Grafana background threads will *not* work reliably, as the Lambda execution environment is halted when it is not processing
any request (Grafana's alerting for example).

# Environment

Configure the following environment variables for the Lamba function:
```
GF_DATABASE_HOST	pg-cluster.cluster-foo.eu-central-1.rds.amazonaws.com
GF_DATABASE_PASSWORD	supersecret!
GF_DATABASE_TYPE	postgres
GF_DATABASE_USER	grafana
GF_SERVER_DOMAIN	grafana.example.com
GF_SERVER_ROOT_URL	https://grafana.example.com
```

# Dockerfile

```
FROM public.ecr.aws/g2o8x4n0/reweb as reweb

FROM grafana/grafana
COPY --from=reweb /reweb /reweb

# if needed
#RUN grafana-cli plugins install grafana-timestream-datasource

ENV GF_PATHS_DATA /tmp

ENV REWEB_APPLICATION_EXEC /run.sh
ENV REWEB_APPLICATION_PORT 3000
ENV REWEB_FORCE_GZIP True
ENV REWEB_WAIT_CODE 302

ENTRYPOINT ["/reweb"]
```

#### Rationale

- Grafana needs to write some data, but it doesn't have to persist -- therefore `/tmp` is fine
- Some Javascript files served by Grafana exceed the 6 MB response limit, hence we need to force gzip compression
- Uncomment / adjust / add `grafana-cli plugins install` commands as necessary
