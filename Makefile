# Makefile for deploying redirector on Google Cloud Platform.
# 
# To customize for personal use:
#   * Change gs_redirector to refer to your own Google Cloud Storage bucket.
#   * make install-redirector
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
gs_server=gs://swtch/server
gs_webroot=gs://swtch/web
gs_certs=gs://swtch/certs

# Build redirector for linux/amd64 and copy to Google Cloud Storage
install-server:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -o swtch.linux .
	gsutil cp swtch.linux $(gs_server)

install-web:
	gsutil -m rsync -r web $(gs_webroot)

# Start VM for rsc.io. The VM name is rsc-io, as is the name for the IP address
# (acquired via 'make newip-rsc-io'). Using the debian-7 image is important
# because it has the startup-script support (Ubuntu does not).
# The f1-micro instance seems to be plenty of power for this use and costs ~$100/year.
# The gce-startup-script file is copied to the VM and runs at startup.
# It reads the three metadata variables at the end of the script, copies the
# redirector from the first one (redirector=) and then invokes it with the
# arguments given by the second and third (import= and repo=).
start-www:
	gcloud compute instances create www --address swtch-com \
		--zone us-central1-a \
		--image debian-7 \
		--metadata-from-file startup-script=gce-startup-script \
		--machine-type f1-micro \
		--metadata \
			server=$(gs_server) \
			webroot=$(gs_webroot) \
			certs=$(gs_certs) \

# Same, but instance and address name is rsc-io-test. For testing.
start-rsc-io-test:
	gcloud compute instances create rsc-io-test --address rsc-io-test \
		--zone us-central1-a \
		--image debian-7 \
		--metadata-from-file startup-script=gce-startup-script \
		--machine-type f1-micro \
		--metadata \
			redirector=$(gs_redirector) \
			import=rsc.io/* \
			repo=https://github.com/rsc/* \
			certs=gs://rsc/certs \

newip-%:
	gcloud compute addresses create $* --region=us-central1

stop-%:
	gcloud compute instances delete $*

ssh-%:
	gcloud compute ssh $*

allow-http:
	gcloud compute firewall-rules create http --description "Incoming http allowed." --allow tcp:80 tcp:443

