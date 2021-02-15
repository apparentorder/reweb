# re:Web
re:Web enables classic web applications to run on AWS Lambda.

re:Web interfaces with the Lambda Runtime API. It translates API Gateway requests back into HTTP requests and passes them to the web application.

# But Why?
Traditional web applications need to be deployed on VMs or in containers. These run continuously around the clock, which means
you have to reserve (pay) CPU and RAM capacity continuously. Every millisecond that your service is not busy handling a web request, it is wasting resources.
Typical web applications spend more than 90% of their CPU time idle. That's a lot of waste!
Finely tuned and well-operated high-traffic applications may see much better utilization, but even then there is a lot of headroom (waste).

With re:Web and AWS Lambda, you can practically eliminate this waste: Resources are billed only as they are actually used for each individual web
request, down to the millisecond.

This can mean significant savings for any usage pattern, but of course it's especially awesome for applications that are not used around the
clock, like Kibana, Grafana, Confluence etc.

And this architecture brings a lot of other benefits for free:
- Seamless auto-scaling without any configuration
- Full high-availability across all Availability Zones
- Easy code updates
- No maintenance required
- Automatic replacement of failed containers

# How it Works

High level overview:

![re:Web arch](https://github.com/apparentorder/reweb/blob/main/doc/reweb-arch.png)

#### API Gateway

We abuse the API Gateway because it's simply the better Load Balancer -- it has less administrative overhead, we don't need to embed it in any
VPC, and most importantly, it charges per actual request instead of per hour.

It is used simply as a dumb HTTP proxy and forwards all requests to Lambda. Note that it terminates TLS; it allows custom domain names and ACM certificates.

#### re:Web

The re:Web binary is a small piece of Go code that is added to the original web application's container image.
It is the Lambda's entrypoint. On startup, it starts the actual web application, and waits until it becomes available.
It handles communication with the Lambda Runtime API, and for each incoming request (Lambda invocation), it will make a
corresponding HTTP request to the web application.

#### Application Server

This is simply the web application, as it would have been deployed per usual.

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
- [Wordpress](https://github.com/apparentorder/reweb/blob/main/doc/app/wordpress.md)
- [Grafana](https://github.com/apparentorder/reweb/blob/main/doc/app/grafana.md)
- [Kibana](https://github.com/apparentorder/reweb/blob/main/doc/app/kibana.md)

Click on the links for setup details.

The following applications are known NOT to work:
- pgAdmin (session management)

# Limitations

Due to the potentially high and fluctuating concurrency of Lambda, re:Web can only work with applications that behave properly in such settings.
Any application that needs to keep *local* state, like session information, will not work. While some such applications can be coerced by using
a load balancer's "sticky session" feature, this workaround will not help on Lambda.

Lambda itself has several very important limitations.

First, any Lambda response cannot exceed 6 MB. That means any web request resulting in a larger response will fail. Common examples are static content like huge
images and enormous Javascript files. Sometimes using `$REWEB_FORCE_GZIP` helps, but that's not guaranteed.

Second, the Lambda environment fully halts execution while there is no request in progress. That means there cannot be any background activity.
This is perfectly fine when the code path is purely request based, for example with PHP. Anything with backgrounds threads, like Java or Node, may
trip because it's being stopped and resumed all the time. In practice, this seems to cause no harm, but it must be kept in mind.

Third, while Lambda can deploy from container images, it's not actually running a container as we know it. One important difference is that *all* the
file system is read-only (except for `/tmp` and `/mnt`). Writes to, say, `/var/run/foo.pid` will fail. Any such paths will need to be adjusted.

Another limitation: API Gateway supports HTTPS only -- no unencrypted HTTP. This shouldn't be a problem nowadays; in fact, it's usually welcome.

One more thing to keep in mind is the web application's startup time. Every time when Lambda needs to spawn an additional instance to handle a
request, this request will have to wait until that instance is ready. This is next to nothing for many languages like PHP and Go (1-3 seconds is not
very noticable), but can be annoying (Kibana/Node takes ~10 seconds to start) or it can flat out stop the show (e.g. Confluence/Java takes about
half a day to start). All subsequent requests that can be handled by a "warm" instance can complete in mere milliseconds, of course.
