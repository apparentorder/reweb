# re:Web
re:Web enables classic web applications to run on AWS Lambda.

re:Web interfaces with the Lambda Runtime API. It translates API Gateway requests back into HTTP requests and passes them to the web application.

# But Why?
Traditionally, web applications run on a VM or in container deployments. These run continuously around the clock, which means
you have to reserve (pay) CPU and RAM capacity continuously. Every millisecond that your service is not busy handling a web request, it is wasting resources.
Typical web applications spend more than 90% of their CPU time idle. That's a lot of waste!
Finely tuned and well-operated high-traffic applications may see much better utilization, but even then there is a lot of headroom (waste).

With re:Web and AWS Lambda, you can practically eliminate this waste: Resources are billed only as they are actually used for each individual web
request, down to the millisecond.

This architecture brings a lot of other benefits:
- Seamless auto-scaling without any configuration
- Full high-availability across all Availability Zones
- Easy code updates
- No maintenance required
- Automatic replacement of failed containers

# How it works

High level overview:

![re:Web arch](https://github.com/apparentorder/reweb/blob/main/doc/reweb-arch.png)

#### API Gateway

We abuse the API Gateway because it's simply the better Load Balancer -- it has less administrative overhead, we don't need to embed it in any
VPC, and most importantly, it charges per actual request instead of per hour.

It is used simply as a dumb HTTP proxy and forwards all requests to Lambda. Note that it terminates TLS; it allows custom domain names and ACM certificates.

#### re:Web

The re:Web binary is a small piece of Go code that is added to the original web application's container image.
It is the Lambda's entrypoint. On startup, it starts the actual web application. It handles communication with the Lambda Runtime API, and for each
incoming request (Lambda invocation), it will make a corresponding HTTP request to the web application.

#### Application Server

This is simply web application, as it would have been deployed per usual.

# Setup

In many cases, the web application's container image will work without modification. We merely need to add the re:Web binary
and add a few environment variables. For example:

```
FROM public.ecr.aws/g2o8x4n0/reweb:latest as reweb

FROM grafana/grafana
COPY --from=reweb /reweb /reweb

ENV REWEB_APPLICATION_EXEC	/run.sh
ENV REWEB_APPLICATION_PORT	3000
ENV REWEB_FORCE_GZIP	True

ENTRYPOINT ["/reweb"]
```

The resulting image is ready for deployment to Lambda.

TODO: full setup walk-through with apigw, lambda, route53, acm

# Applications

The following applications have been tested and are known to work:
- ...

# Limitations
- ...
