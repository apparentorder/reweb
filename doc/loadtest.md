# Some Test Data...

Hitting a Wordpress front page using the wonderful [siege](https://github.com/JoeDog/siege/) load testing tool.

Siege will use a somewhat realistic pattern of 20 concurrent users each loading the page every 10 seconds.
Every page load translates to several HTTP requests, as it requests dependent URLs.

This amounts to ~40 requests per second, or ~2,400 requests per minute, or ~3.5M requests per day.

```
# siege -c20 -d10 -t30m -v https://wpdemo.example.com/
{	"transactions":			       71161,
	"availability":			      100.00,
	"elapsed_time":			     1799.07,
	"data_transferred":		      812.31,
	"response_time":		        0.05,
	"transaction_rate":		       39.55,
	"throughput":			        0.45,
	"concurrency":			        2.10,
	"successful_transactions":	       71162,
	"failed_transactions":		           0,
	"longest_transaction":		        4.61,
	"shortest_transaction":		        0.01
}
```

Lambda concurrency was surprisingly low, between 6 and 9 over the whole test.

If we were to extrapolate that to a full *day* of traffic, we'd have ~3.5M requests (maybe 500.000 page impressions? --
many German newspapers have less than that!).

This test traffic averages to ~40 milliseconds of Lambda compute per request (front page is ~250msec and static assets are more like 5msec each).

For Lambda:

40ms * 3.5M requests * [$0.0000000083 per ms at 0.5 GB](https://aws.amazon.com/lambda/pricing/) is about ~$1.16 per day.

For API Gateway:

3.5M requests * [$0.0000012 per request](https://aws.amazon.com/api-gateway/pricing/) is about $4.2 per day.

So there you have an extremly high-available high-traffic Wordpress site with superb response times at ~$5 per day.

That is a *huge lot* of traffic that most Wordpress installations will never see, not even close. And for every request
less than that, you'll pay less.

And also keep in mind that this is *without* any caching by a browser or by any CDN. If there'd been some CDN, pretty
much all static requests would never hit the Lambda function, reducing the request count to maybe *one fifth* of
the numbers above.

Also note we're not counting AWS egress traffic or RDS/Aurora costs. The database tends to be bored anyway -- for the above
test, Aurora Serverless with just 1 ACU was idling at 10-15% CPU. EFS cost for a few dozen MBs is negligible (below $0.000/day).
