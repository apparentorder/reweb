# Overview

Works pretty great!

- Needs some MySQL for sessions etc. (Aurora Serverless works)
- Needs EFS as the Wordpress directory needs to be shared and writable
- EFS should map all access to UID/GID 33/33 (corresponds to `www-data` in the Wordpress image)
- Needs private VPC connectivity for EFS (and Aurora Serverless, if used)
  - mostly works without Internet connectivity (NAT), but some Admin functions will fail and the Dashboard will load slowly
- Lambda with 512 MB memory seemst to be fine and fast enough (usually <150 MB)
  - using 3072 MB (one full vCPU) is slightly faster but not worth it (most time is spent waiting for the database)
- Initial Wordpress setup cannot be done from Lambda (the Wordpress image tries to un`tar` the whole thing)
  - Make sure the following two things are set in `wp-config.php`:
    ```
    define('WP_MEMORY_LIMIT', '450M');
    
    /* HTTPS detection via proxy headers */
    if (!empty($_SERVER['HTTP_X_FORWARDED_PROTO']) && $_SERVER['HTTP_X_FORWARDED_PROTO'] === 'https') {
        $_SERVER['HTTPS'] = 'on';
    }
    ```

# Dockerfile

```
FROM public.ecr.aws/g2o8x4n0/reweb as reweb

FROM wordpress:5.6 as wordpress
COPY --from=reweb /reweb /reweb

RUN sed -i s/80/8080/ /etc/apache2/ports.conf /etc/apache2/sites-enabled/000-default.conf
RUN sed -i s,/var/www/html,/mnt/wordpress, /etc/apache2/sites-enabled/000-default.conf
RUN sed -i s,/var/www,/mnt/wordpress, /etc/apache2/apache2.conf

ENV REWEB_APPLICATION_EXEC cd /mnt/wordpress && docker-entrypoint.sh apache2-foreground
ENV REWEB_APPLICATION_PORT 8080
ENV APACHE_PID_FILE /tmp/httpd.pid
ENV APACHE_RUN_USER ec2-user

ENTRYPOINT ["/reweb"]
```

#### Rationale

- Default port needs to be changed from 80
- `/var/www/html` is the Wordpress installation directory -- this needs to be on EFS, and EFS can only be mounted below `/mnt`
- Wordpress is simply looking in the current working directory, hence the `cd` before calling the original entry point
- PID needs to go to `/tmp`, as the usual directory in `/var` cannot be written to
- The user ID needs to exist (the Wordpress image uses `www-data`, but the Lambda environment does *not* use this `/etc/passwd`)
