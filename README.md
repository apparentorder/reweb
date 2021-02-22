# re:Web
re:Web enables classic web applications to run on AWS Lambda.

re:Web interfaces with the Lambda Runtime API. It translates API Gateway requests back into HTTP requests and passes them to the web application.

Due to this generic mechanism, it works with *any* web application that can be load-balanced properly.

## But Why?
Traditional web applications need to be deployed on VMs or in containers. These run continuously around the clock, which means
you have to reserve (pay) CPU and RAM capacity continuously. Every millisecond that your service is not busy handling a web request, it is wasting resources.
Typical web applications spend more than 90% of their CPU time idle. That's a lot of waste!
Finely tuned and well-operated high-traffic applications may see much better utilization, but even then there is a lot of headroom (waste).

With re:Web and AWS Lambda, you can practically eliminate this waste: Resources are billed only as they are actually used for each individual web
request, down to the millisecond.

This can mean significant savings for any usage pattern, but of course it's especially awesome for applications that are not used around the
clock, like Kibana, Grafana, Confluence etc.

This architecture has some key benefits:
- It's *significantly* cheaper for many workloads
- Seamless auto-scaling without any configuration
- Full high-availability across all Availability Zones
- Easy code updates

## Status

This is experimental / a proof of concept. Maybe don't use it in a high-profile production site just yet. :-)

But it works surprisingly well. I'd like to evolve this idea and push the envelope a little, to see what's possible.
The most imporant next step is to test and document more applications that work this way.

## Applications

Because re:Web behaves like a HTTP proxy, it can potentially work with many, many applications!

It requires zero code changes.

All you need to do is to add the re:Web binary and use it as the `ENTRYPOINT`.
In some cases, some trivial changes to the application's `Dockerfile` are necessary, to make the application suitable
for the AWS Lambda execution environment.

The following applications have been tested and are known to work:
- Wordpress ([setup reference](doc/app/wordpress.md)) ([full setup walk-through](doc/app/wordpress-setup.md))
- SMF / Simple Machines Forum ([setup reference](doc/app/smf.md))
- Grafana ([setup reference](doc/app/grafana.md))
- Kibana ([setup reference](doc/app/kibana.md))

Click on the links for setup details.

The following applications are known NOT to work:
- pgAdmin (session management)

### More Data

There's some [high-traffic load test data](doc/loadtest.md) for Wordpress; it includes some napkin math for potential AWS costs.

## How it Works

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

This is simply the web application, as it would have been deployed per usual. Most software images come with some web server built-in, e.g. Apache or nginx,
and/or they provide their own HTTP server which would serve traffic directly to the public in a VM or container deployment. re:Web acts as a proxy to
this HTTP server.

## Future Ideas

Many! In no particular order:
- Test and document many many more applications!
- Work around the Lambda 6 MB response limitation by dynamically offloading such responses to S3
- Implement re:Web for AWS Application Loadbalancer (as alternative to API Gateway), because the former becomes cheaper at some level of traffic
- Provide ready-to-use images of popular applications
- Provide Terraform and/or Cloudformation packages for one-click deployment
- Find a good way to provide secret data as environment variables (from Secrets Manager or SSM Parameter Store, but without impacting cold-start time)
- Find ways to give more "breathing room" for background threads ([ideas](https://www.reddit.com/r/aws/comments/lnjo6z/run_classic_web_applications_on_aws_lambda_and/go2fuqb/?utm_source=reddit&utm_medium=web2x&context=3))
- Your ideas...?

Contributions welcome, of course! See below for "Contact".

## Limitations

#### Session management

Due to the potentially high and fluctuating concurrency of Lambda, re:Web can only work with applications that behave properly in such settings.
Any application that needs to keep *local* state, like session information, will not work. While some such applications can be coerced by using
a load balancer's "sticky session" feature, this workaround will not help on Lambda.

#### Lambda Limitations

Lambda itself has several very important limitations.

- Any Lambda response cannot exceed 6 MB. That means any web request resulting in a larger response will fail. Common examples are static content like huge
images and enormous Javascript files. Sometimes using `$REWEB_FORCE_GZIP` helps, but that's not guaranteed.

- The Lambda environment fully halts execution while there is no request in progress. That means there cannot be any background activity.
This is perfectly fine when the code path is purely request based, for example with PHP. Anything with backgrounds threads, like Java or Node, may
trip because it's being stopped and resumed all the time. In practice, this seems to cause no harm, but it must be kept in mind.

- While Lambda can deploy from container images, it's not actually running a container as we know it. One important difference is that *all* the
file system is read-only (except for `/tmp` and `/mnt`). Writes to, say, `/var/run/foo.pid` will fail. Any such paths will need to be adjusted.

- Lambda does not allow root-level privileges, therefore it's not possible to use well-known ports -- any web application that comes with a default
port below 1024 needs to be reconfigured.

#### API Gateway limitations

API Gateway supports HTTPS only -- no unencrypted HTTP. This shouldn't be a problem nowadays; in fact, it's usually welcome.

API Gateway's maximum timeout is 30 seconds, so any request longer than this will fail.

#### Startup Time

One more thing to keep in mind is the web application's startup time. Every time when Lambda needs to spawn an additional instance to handle a
request, this request will have to wait until that instance is ready.

This is next to nothing for many languages like PHP and Go (1-3 seconds at most -- not very noticable), but can be annoying (Kibana/Node takes
~10 seconds to start) or it can flat out stop the show (e.g. Confluence/Java takes about half a day to start). All subsequent requests handled by a "warm" instance can complete in mere milliseconds, of course.

This can be worked around with using Lambda Provisioned Concurrency, but that voids the Lambda cost advantage. It might still be preferable to a
container deployment for availability reasons though; "it depends".

#### Lambda VPC Interface Idle

To quote an [AWS blog post](https://aws.amazon.com/de/blogs/compute/announcing-improved-vpc-networking-for-aws-lambda-functions/):
"If Lambda functions in an account go idle for *consecutive weeks*, the service will reclaim the unused Hyperplane
resources and so very infrequently invoked functions may still see longer cold-start times" (emphasis added)

## Related Work

- [serverlessish](https://github.com/glassechidna/serverlessish), a project that is ["eerily similar"](https://twitter.com/__steele/status/1363020663615758339). It really is.

- [Serverless WordPress on AWS Lambda](https://keita.blog/2020/06/29/wordpress-on-aws-lambda-efs-edition/) modifies Wordpress to run in Lambda,
giving basically the same results as re:Web for this specific application. The article has some additional hints regarding S3 plug-ins.

- [Bref](https://github.com/brefphp/bref) "helps you deploy PHP applications to AWS and run them on AWS Lambda". It's built on the [Serverless Framework](https://www.serverless.com). ([Reddit comment](https://www.reddit.com/r/aws/comments/lnjo6z/run_classic_web_applications_on_aws_lambda_and/go1zacd/?utm_source=reddit&utm_medium=web2x&context=3))

- [Amazon.Lambda.AspNetCoreServer](https://www.nuget.org/packages/Amazon.Lambda.AspNetCoreServer/) "makes it easy to run ASP.NET Core Web API applications as AWS Lambda functions" ([AWS Blog Post](https://aws.amazon.com/blogs/developer/running-serverless-asp-net-core-web-apis-with-amazon-lambda/))

- [Chalice](https://github.com/aws/chalice) "is a framework for writing serverless apps in python."

- "My Dream of Truly Serverless", a blog post I have yet to write

#### A Tale of Two Projects

As mentioned above, the *serverlessish* implements the very same idea, in an amazingly similar way. The main differences are that it is built for the
AWS Application Loadbalancer (instead of API Gateway) and is designed as a Lambda Extension. We have been in contact; updates soon.

## Are you AWS?

I believe the whole concept here is gold. But shoehorning it into Lambda requests and translating JSON/HTTP back and forth is hacky. This could be made into
something beautiful, with some changes in the involved AWS Services. Let's talk!

## Contact

For bug reports, pull requests and other issues please use Github.

For everything else:
- I'm trying to get used to Twitter as [@apparentorder](https://twitter.com/apparentorder). DMs are open.
- Sometimes I peek into the og-aws Slack, as "appo"
- I'm old enough to prefer IRC -- find me in #reweb on Freenode.
- Last resort: Try legacy message delivery to apparentorder@neveragain.de.
