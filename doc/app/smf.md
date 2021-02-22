# Overview

Seems to run fine at first glance, but not tested extensively.

- Needs some MySQL for sessions etc. (Aurora Serverless works)
- Needs EFS as the SMF directory needs to be shared and writable
- EFS needed
- Needs private VPC connectivity for EFS (and Aurora Serverless, if used)
- Lambda with 512 MB memory seemst to be fine and fast enough (usually <100 MB) -- maybe even less would be fine
- Initial setup cannot be done from Lambda

To install, create an EFS Access Point (map all access to a single UID/GID). Use an EC2 instance to unpack the SMF installation).
From there, use use `https://yourURL/install.php` to run the installation. Be sure to adjust the forum URL from `http://` to `https://` when asked.

The complete setup procedure is basically the same as Wordpress, so you can refer to that [walk-through](wordpress-setup.md).

# Dockerfile

```
FROM public.ecr.aws/apparentorder/reweb as reweb

FROM php:7.4-apache
COPY --from=reweb /reweb /reweb

ENV APACHE_DOCUMENT_ROOT /mnt/smf

RUN sed -i s/80/8080/ /etc/apache2/ports.conf /etc/apache2/sites-enabled/000-default.conf
RUN sed -ri -e 's!/var/www/html!${APACHE_DOCUMENT_ROOT}!g' /etc/apache2/sites-*/*.conf
RUN sed -ri -e 's!/var/www/!${APACHE_DOCUMENT_ROOT}!g' /etc/apache2/apache2.conf /etc/apache2/conf-*/*.conf

RUN docker-php-ext-install mysqli pdo_mysql

ENV REWEB_APPLICATION_EXEC cd /mnt/smf && docker-php-entrypoint apache2-foreground
ENV REWEB_APPLICATION_PORT 8080
ENV APACHE_PID_FILE /tmp/httpd.pid
ENV APACHE_RUN_USER ec2-user

ENTRYPOINT ["/reweb"]
```

#### Rationale

- SMF does not provide any containers; we use the generic php-apache image here
- Default port needs to be changed from 80
- PID needs to go to `/tmp`, as the usual directory in `/var` cannot be written to
