slicesync
=========

Slicesync syncs slices of files using rsync philosophy (but not the same exact algorithm)

It is inspired in both rsync (http://rsync.samba.org/) and zsync (http://zsync.moria.org.uk/).

The idea is to cover the following features:
- Server side can be a simple HTTP server with Range support (just as zsync)
- Files to be synced NEED to have a hash dump file produced in advance (similar to a zsync .zsync files)
- Server side hash dumps are prepared by a simple hashing service on the background.
- Client side does the heavy processing part (as zsync)
- When there is no local file to sync to, it defaults to a simple direct download.
- All syncs but the direct download check the file downloaded hash (currently SHA1)
- All downloads should bring the server-side pre-generated hash dump file, to speed up later syncs. [Pending]
