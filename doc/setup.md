# Full Setup

This page will describe all the setup steps required to get a full Wordpress site up & running on re:Web.

We will be using `wpdemo.cmdu.de` as the site name. Adjust for your name.

# Prerequisites

We'll assume the following things are already there:
- VPC (possibly with a NAT Gateway or NAT instance for connectiviy, though not strictly required)
- Possibly a Security Group for all this (we'll use the VPC default SG here)
- MySQL Database (e.g. RDS or Aurora Serverless)
- Custom domain in Route53 (only needed if you want the site under your own domain name)
- some EC2 instance 
  - with Docker installed
  - with access to the same VPC you'll be using
  - with an EC2 Instance Role that allows to use ECR (you can use the Managed Policy "AmazonEC2ContainerRegistryPowerUser")

# Create the EFS

- Open the EFS console
- "Create filesystem"
  - Name it "demo" or whatever
  - Make sure the correct VPC is selected, create it
- Select "Access points" and create one
  - Name it "wpdemo", at path "/wpdemo"
  - for "POSIX user", put 33 as both UID and GID
  - same for "Root directory creation permissions", the "mode" is 0755
- Click the created Access Point to view the details
- Click "Attach" and note the `mount` suggested command under "Using the EFS mount helper"

# Build the Docker Image

On your EC2 instance, do this:

- `aws ecr get-login-password | docker login --username AWS --password-stdin 123456789012.dkr.ecr.eu-central-1.amazonaws.com` (use your AWS Account ID and Region!)
- `aws ecr create-repository --repository-name wpdemo` -- note the `registryUri`!
- `mkdir wpdemo`
- `cd wpdemo`
- `${EDITOR-vi} Dockerfile` -- insert the Dockerfile from [the Wordpress docs](app/wordpress.md)
- `docker build -t 123456789012.dkr.ecr.eu-central-1.amazonaws.com/wpdemo:latest .` -- note the `.` at the end; replace your AWS Account ID and Region, so it matches your `registryUri`
- `docker push     123456789012.dkr.ecr.eu-central-1.amazonaws.com/wpdemo:latest` (replace again)

# Deploy Wordpress to EFS

- Mount the EFS filesystem to the EC2 instance using the command noted earlier, something like `sudo mount -t efs -o tls,accesspoint=fsap-0da9351c6f768c74a fs-98f6fbc0:/ efs`
  - Note that you'll need to replace the last `efs` with your actual mount point, e.g. `/mnt`

((( to be continued )))

# Custom Domain Name Setup

Feel free to skip this section if you don't want a custom domain name. You can always add it later.

First off, we'll request the TLS certificate for our chosen name:
- Open the Certificate Manager Console in your region
- Request public certificate with domain name `wpdemo.cmdu.de` (using DNS validation)
- Expand the Validation thingie and click "Create record in Route53" to have ACM create the necessary CNAME entry
- Wait a bit until the certificate changes status to "Issued" (just a few seconds usually, but could take some minutes)

# API Gateway

- Open API Gateway
- Hit "Create API"
- Choose to "Build" a "HTTP API"
- Name it "wpdemo"
- Click through the following screens, not adding anything further / accepting defaults
- On the left, select "Integrations"


note cloudfront acm cert region
