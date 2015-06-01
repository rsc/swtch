# Makefile for deploying redirector on Google Cloud Platform.
# 
# To customize for personal use:
#   * Change gs_server to refer to your own Google Cloud Storage bucket.
#   * make install-server
#
# To add a new host:
#   * Copy the start-rsc-io stanza and customize VM name, address name, import= and repo=.
#   * Adjust certs= to point to a gs:// directory containing <host>.crt and <host>.key,
#     or delete that metadata entry to disable serving with HTTPS.
#   * make newip-your-vm to get an IP address.
#   * make start-your-vm to start the VM.
#
# To ssh into a host (to debug):
#   * make ssh-your-vm
# 
# To restart a host:
#   * make stop-your-vm
#   * make start-your-vm

# gs:// URL for Google Cloud Storage location of redirector binary.
gs_server=gs://swtch/server.cloud
gs_webroot=gs://swtch/www
gs_certs=gs://swtch/certs

# Build redirector for linux/amd64 and copy to Google Cloud Storage
install-server:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -o swtch.linux .
	gsutil cp swtch.linux $(gs_server)

install-cron-exe: install-cron-cl2issue install-cron-godash install-cron-issue install-cron-cl install-cron-gcscat

install-cron-cl2issue:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -o cl2issue.linux rsc.io/github/cl2issue
	gsutil cp cl2issue.linux gs://swtch/cron/exe/cl2issue

install-cron-godash:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -o godash.linux rsc.io/github/godash
	gsutil cp godash.linux gs://swtch/cron/exe/godash

install-cron-issue:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -o issue.linux rsc.io/github/issue
	gsutil cp issue.linux gs://swtch/cron/exe/issue

install-cron-cl:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -o cl.linux golang.org/x/build/cmd/cl
	gsutil cp cl.linux gs://swtch/cron/exe/cl

install-cron-gcscat:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -o gcscat.linux rsc.io/cloud/google/gcscat
	gsutil cp gcscat.linux gs://swtch/cron/exe/gcscat
	
install-crontab:
	gsutil cp crontab gs://swtch/cron
	gsutil cp updateissue updatedash gs://swtch/cron/exe
	gsutil cp crongo gs://swtch/cron/exe/go

# install-web:
# 	gsutil -m rsync -r web $(gs_webroot)

# Start VM for swtch.com. The VM name is swtch-com, as is the name for the IP address
# (acquired via 'make newip-swtch-com'). Using the debian-7 image is important
# because it has the startup-script support (Ubuntu does not).
# The f1-micro instance seems to be plenty of power for this use and costs ~$100/year.
# The gce-startup-script file is copied to the VM and runs at startup.
# It reads the metadata variable named server, fetches that executable from
# Google Cloud storage as /work/server, and runs it in /work.
# The server itself initializes its flag from metadata variables before
# processing the command line.
start-www:
	gcloud compute instances create www --address swtch-com \
		--zone us-central1-a \
		--image debian-7 \
		--metadata-from-file startup-script=gce-startup-script \
		--machine-type f1-micro \
		--metadata server=$(gs_server) \
			host=swtch.com \
			cron=gs://swtch/cron \
			tls=true \

# Same, but instance and address name is www-test. For testing.
start-www-test:
	gcloud compute instances create www-test --address www-test \
		--zone us-central1-a \
		--image debian-7 \
		--metadata-from-file startup-script=gce-startup-script \
		--machine-type f1-micro \
		--metadata \
			server=$(gs_server) \
			host= \

newip-%:
	gcloud compute addresses create $* --region=us-central1

stop-%:
	gcloud compute instances delete $*

ssh-%:
	gcloud compute ssh $*

allow-http:
	gcloud compute firewall-rules create http --description "Incoming http allowed." --allow tcp:80 tcp:443

# Create signed URLs to allow the cron job in updatedash to update the dashboards,
# without having general write access to the gs://swtch/ bucket.
signurls:
	gsutil signurl -p notasecret -m PUT -d 1000d swtch-com-*.p12 gs://swtch/www/godash/cl | sed -n 's;^.*https://;https://;p' >update-cl.url
	gsutil signurl -p notasecret -m PUT -d 1000d swtch-com-*.p12 gs://swtch/www/godash/release | sed -n 's;^.*https://;https://;p' >update-release.url
	gsutil cp update-*.url gs://swtch/cron/
