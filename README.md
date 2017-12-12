# droplet-registry-spike

A docker registry that converts Cloud Foundry apps to docker images on demand.
See [this related Pivotal Tracker
story](https://www.pivotaltracker.com/story/show/151941576).

## Usage

1. Perform these steps on a Linux host that has the docker daemon running.
1. Download [a release of the cflinuxfs2 Cloud Foundry
   rootfs](https://github.com/cloudfoundry/cflinuxfs2/releases).
1. Log into a Cloud Foundry installation, and push an app.
1. Run the registry, e.g.: `go run *.go --store <some cache path> --capi-url
   https://api.<CF system domain --capi-authtoken $(cf oauth-token)
   --listen-address 127.0.0.1:8080 --rootfs-path <cflinuxfs2.tar.gz>`
1. After a few seconds, the rootfs will be imported and the API will begin
   listening.
1. `docker pull 127.0.0.1:8080/$(cf app <name> --guid)`
1. Alternatively, `docker run -it --rm 127.0.0.1:8080/$(cf app <name> --guid)
   /bin/bash`.

## What's going on when we pull an image?

The rootfs is simply copied into the store, named in a content-addressable way
(after its own sha256 checksum).

The droplet is downloaded and cached. Each tar entry in the droplet has a
relative pathname.  The registry re-writes the tar to disk, modifying the
header metadata to convert these header pathnames to absolute ones, anchoring
them at `/home/vcap`, because this is where they would be un-tarred by the
Cloud Foundry runtime. It's also stored in a content-addresssable way.

When the docker daemon reads the manifest returned for the image, it will then
request the config and both layers as blobs. The registry will redirect these
requests to some non-docker-API endpoint, which for now is on the same server.
This is discussed further in the "Learnings" section below.

## Limitations and possible future work

1. Since this is a spike and I'm lazy, you have to pass in a valid UAA OAuth
   token. A non-toy implementation of this would fetch it's own auth token
   using appropriately-scoped UAA client credentials.
1. It always pulls the latest droplet. Future implementations could take a
   droplet ID using the docker tag.
1. The registry is not highly available. This is discussed more in the
   "Learnings" section below.
1. We pull using the app guid, not name. Future implementations could look up
   the GUID using the CF API.

## Learnings

This worked pretty well. The app layer is purely additive, so rewriting the
pathname in the tar header metadata for each tar entry was all that was needed.

In a non-toy implementation, we'd need to ensure high availability. We don't
want to reinvent the HA blobstore, and most production CF deployments use
external (and usually S3-compatible) blobstores. Each CF component that wants
to talk to the blobstore uses [fog](https://github.com/fog/fog) to smooth over
the inevitable differences between S3 "compatible" blobstores.

Another option is to use the (still experimental) bits-service. [Its
API](http://cloudfoundry-incubator.github.io/bits-service/) has endpoints for
each CF domain entity that it knows about, so we'd probably want to add one for
docker/OCI layers. We didn't manage to get bits-service working in an
externally-reachable fashion, but this probably isn't a huge risk since you can
see the docker daemon following HTTP 307 redirects, so we know we can keep the
blobs/image config in another location.

As for the manifests themselves, they are small in size, and can be stored in
one of the stores CF already uses, e.g. some failover-configured RDBMS (could
be another schema in the same physical instance as CCDB), or something like
etcd / Redis.

Alternatively, the manifest cache could be instance-local. If it's cached on
the instance handling the request, great. If not, then as long as it checks the
blobstore / bits-service for the presence of the checksummed blob in question
before trying to re-upload it, then the performance shouldn't be too bad.
