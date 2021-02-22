# Full Setup

The fundamental steps to set up some application with re:Web in Lambda:
- Build a custom container image, to add re:Web
- Deploy this image to an AWS ECR repository
- Create a Lambda function from this repository
  - possibly connect VPC, EFS etc. as required
- Create an API Gateway, proxying all traffic to that Lambda function
- optionally, add a custom domain to API Gateway

This page will describe all the setup steps required to get a full Wordpress site up & running on re:Web, including EFS.

We will be using `wpdemo.cmdu.de` as the site name. Adjust for your name.

# Prerequisites

We'll assume the following things are already there:
- VPC (possibly with a NAT Gateway or NAT instance for connectivity, though not strictly required)
- Possibly a Security Group for all this (we'll use the VPC default SG here)
- MySQL Database (e.g. RDS or Aurora Serverless)
- Custom domain / hosted zone in Route53 (only needed if you want the site under your own domain name)
- some EC2 instance 
  - with Docker installed
  - with access to the same VPC you'll be using
  - with an EC2 Instance Role that ...
    - allows to use ECR (e.g. Managed Policy "AmazonEC2ContainerRegistryPowerUser")
    - allows EFS Clients (e.g. Managed Policy "AmazonElasticFileSystemClientReadWriteAccess")

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
- Click "Attach" and note the suggested `mount` command under "Using the EFS mount helper"

# Build the Docker Image

On your EC2 instance, do this:

- `aws ecr get-login-password | docker login --username AWS --password-stdin 123456789012.dkr.ecr.eu-central-1.amazonaws.com` (use your AWS Account ID and Region!)
- `aws ecr create-repository --repository-name wpdemo` -- note the `registryUri`!
- `mkdir wpdemo`
- `cd wpdemo`
- `${EDITOR-vi} Dockerfile` -- insert the Dockerfile from [the Wordpress docs](wordpress.md)
- `docker build -t 123456789012.dkr.ecr.eu-central-1.amazonaws.com/wpdemo:latest .` -- note the `.` at the end; replace your AWS Account ID and Region, so it matches your `registryUri`
- `docker push     123456789012.dkr.ecr.eu-central-1.amazonaws.com/wpdemo:latest` (replace again)

# Deploy Wordpress to EFS

- Mount the EFS filesystem to the EC2 instance using the command noted earlier, something like `sudo mount -t efs -o tls,accesspoint=fsap-0da9351c6f768c74a fs-98f6fbc0:/ efs`
  - Note that you'll need to replace the last `efs` with your actual mount point, e.g. `/mnt`
- `cd /mnt`
- `wget https://wordpress.org/latest.tar.gz`
- `tar --strip-components 1 -zxvf latest.tar.gz`

# Create Lambda Function

In the Lambda Console, "Create function":
- ... from "Container image"
- Name "wpdemo"
- Click "Browse images" to select your "wpdemo" repository and image
- Make sure that "Create new role ..." is selected under "Change default execution role"
- Hit "Create function"
- In "Basic settings", up the RAM to 512 MB and the Timeout to 30 seconds
- Also in "Basic settings", use the "View the wpdemo-role-foo" link at the bottom to open the IAM console
  - Hit "Attach policies" and select the "AWSLambdaVPCAccessExecutionRole"
  - close the tab to return to the Lambda settings page
- Edit VPC settings and place the Lambda function in your correct VPC
  -  be sure to use the correct "private" subnets if you want NAT connectivity
- "Add file system" to connect the EFS
  - select your file system and the "wpdemo" access point
  - "Local mount path" is /mnt/wordpress

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
- On the left, select "Routes", hit "Create"
  - leave method as "ANY"
  - use `/{proxy+}` as the path
  - hit "Create"
- After creating, select the newly created route (the "ANY" link) and "Attach integration"
  - "Integration target" is Lambda
  - in "Integration details", select the "wpdemo" Lambda function
  - Make sure "Grant API Gateway permission..." is selected (default)
  - "Create"
- Go back to the overview of your API (left-hand side, "API: wpdemo (yourApiID)"
  - Note and/or open the "Invoke URL"

#### Custom Domain Name Setup, Part 2

Feel free to skip this section if you don't want a custom domain name. You can always add it later.

Still in API Gateway, on the left, select "Custom domain names" and hit "Create"
- "Domain name" is `wpdemo.cmdu.de`
- Leave the "Endpoint type" at "Regional" and select the correct "ACM certificate"
- "Create domain name"
- Note the resulting "API Gateway domain name"
- Click the "API mappings" tab and "Configure API mapping" and there "Add new mapping"
  - select your "wpdemo" API and the "$default" Stage
  - don't enter a Path
  - "Save"

In the Route53 Console,
- select "Hosted Zones" and your correct domain name
- "Create record"
  - Enter the correct name ("wpdemo")
  - enable the "Alias" toggle (top right corner)
  - In "Route traffic to", select "Alias to API Gateway API", select region and the domain name from before
  - ... "Create records"

# Drumroll!

If you're using your own DNS name, you may need to wait a few minutes until the DNS entry works.

If you're not using your own name, or if you don't want to wait, open the API Gateway "Invoke URL" noted earlier.
Wordpress will start, but it will send you a Redirect to http:// (no TLS), which will cause your browser to throw an
error!

In the browser's address bar, replace `http://` with `https://` (it should now look something
like `https://c1zy14kkai.execute-api.eu-central-1.amazonaws.com/wp-admin/setup-config.php`).

Click "Let's go" and follow the setup. Don't mind the ugly look, this will be fixed soon.

Enter your database information as usual. (Know and rest assured: You actually *do* transmit these data over HTTPS/TLS)

***IMPORTANT BREAK HERE***

Now go back to your EC2 instance, where you un`tar`ed Wordpress. We need to edit your `/mnt/wp-config.php`,
which has now been created, manually. Somewhere after the database credentials, add the following lines:
```
/* HTTPS detection via proxy headers */
if (!empty($_SERVER['HTTP_X_FORWARDED_PROTO']) && $_SERVER['HTTP_X_FORWARDED_PROTO'] === 'https') {
    $_SERVER['HTTPS'] = 'on';
}
```
This makes sure that Wordpress will now know to use `https://` links.

Return to browser and then "Run the installation". In the "Welcome" page, enter the Site information as usual. Hit "Login".

In "Settings" > "General", adjust your "Wordpress Address" and "Site Address" to match your custom DNS name, if any.

Done!

# Homework

Implement all this in Terraform / CloudFormation. 🙃
