This directory holds the App Engine app for `swtch.com`.

It serves a file tree stored in Google Cloud Storage at `gs://swtch/www/`,
including ETag and Last-Modified headers in responses and
implementing If-None-Match, If-Range, and byte range request headers.

Like in the standard Go http.FileSystem implementation:

 - requests for dir/ are served using dir/index.html
 - requests for dir/index.html are redirected to dir/
 - requests for dir are redirected to dir/

A "dir" in this case is defined as a path for which dir/index.html exists.

There is (intentionally) no support for directory listings.

If gs://swtch/www/404.html exists,
its content is used as the response body for any 404 error.

## Certificate recipes

The certificate for swtch.com is a Comodo Wildcard PositiveSSL cert purchased via cheapsslsecurity.com. 

Create a new key:

    $ openssl genrsa -out swtch.com.key 2048
    Generating RSA private key, 2048 bit long modulus
    ..................................+++
    ...............................................+++
    e is 65537 (0x10001)
    $ 

Create a new CSR:

    $ openssl req -new -sha256 -key swtch.com.key -out swtch.com.csr
    You are about to be asked to enter information that will be incorporated
    into your certificate request.
    What you are about to enter is what is called a Distinguished Name or a DN.
    There are quite a few fields but you can leave some blank
    For some fields there will be a default value,
    If you enter '.', the field will be left blank.
    -----
    Country Name (2 letter code) [AU]:US
    State or Province Name (full name) [Some-State]:MA
    Locality Name (eg, city) []:Cambridge
    Organization Name (eg, company) [Internet Widgits Pty Ltd]:swtch.com
    Organizational Unit Name (eg, section) []:
    Common Name (e.g. server FQDN or YOUR name) []:*.swtch.com
    Email Address []:
    
    Please enter the following 'extra' attributes
    to be sent with your certificate request
    A challenge password []:
    An optional company name []:
    $ 

After obtaining Comodo zip file, check that obtained cert matches key:

    $ openssl x509 -noout -modulus -in STAR* | openssl md5
    (stdin)= 788dfd0f7fd196155d32d0cd413fe03d
    $ openssl rsa -noout -modulus -in swtch.com.key | openssl md5
    (stdin)= 788dfd0f7fd196155d32d0cd413fe03d
    $ 

Prepare the concatenated PEM to upload to the Google Cloud Console using:

    cat STAR_swtch_com.crt COMODORSADomainValidationSecureServerCA.crt COMODORSAAddTrustCA.crt >all.crt

For debugging the chain, to dump an individual cert:

    openssl x509 -text -in x.crt

## App Engine Domains

App Engine Domain dashboard at
[[https://console.cloud.google.com/appengine/settings/domains?project=calcium-vector-91212&authuser=2]].
