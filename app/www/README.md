This directory holds the Cloud Run app for `swtch.com`.

	gcloud run deploy www \
		--project=calcium-vector-91212 \
		--region=us-east1 \
		--revision-suffix=123 \
		--source=.

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
