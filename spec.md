Spec
====

SliceSync
---------

SliceSync goal is to allow efficient big file versions updates from an standard HTTP server or the slicesync's syncserver itself.

To make it as fast and efficient as possible while not impossing too much load on the server side, slicesync prepares a bulk hash dump (.slicesync) file for each file to be sent from the server. That file contains some information of the file to be downloaded, most importantly, the hash of each of the slices of that file and the full file hash. That info only needs to be re-calculated when the original file changes.


Client
------

Slicesync client tool is given:

- A .slicesync file URL containg the information to be downloaded
- An optional destination filename
- An optional alike local file
- An optional slice size (it defaults to the slice size in the downloaded .slicesync file)

The client tool then:

1. Downloads the remote .slicesync usage the URL
2. Load or generate on the fly the local alike .slicesync to compare with
3. Calculate the differences
4. Rebuild the remote file by mixing local available parts with remote parts
5. At the end the generated file hash is compared with the remote file hash on .slicesync


Server
------

On the server side there are various options:

1. Use syncserver to serve the files over HTTP.
2. Use any HTTP Range compliant server + "shash -service" on the background.

In any case the files are served per directory or directory tree (recursively). Either syncserver or shash -service will be populating .slicesync files for any new file that is copied to the managed directory or directory tree. Also, files deleted make the corresponding .slicesync file disappear.

To keep the filesystem as clean as possible, all .slicesync files are placed within a single .slicesync directory, one per managed directory tree. Similar to what git or mercurial do with their .git or .hg directories.

Apart from that the server just needs to honour HTTP Range requests properly so that the client only gets the new parts not know before of the files to be downloaded. The server provides access to both the .slicesync files and the actual desired files.


Format of .slicesync files
--------------------------

Following zsync inspiration at http://zsync.moria.org.uk/paper/ch04s02.html, .slicesync files have the following header:

    Version: 0.0.1
    Filename: somefile.extension
    Slice: 1048576
    Slice Hashing: adler32+md5
    Length: 4294967296

Then there will be a Length/Slice lines containing the hashes of each slice in base64 format. Something like these:

    ...
    6qWWSLG/+zAezwliHWLy1Lhujek=
    x4wfZ+l0YY1Xv4muIyIcl2H7flM=
    xRzZX+ks0GHrR1KDtvpBVDxQAdQ=
    at+uJQMpVjfJYl2aUD4dJUSnmrc=
    883V60Xi3Q627Euv9AcXHK7nS1w=
    ...

And finally, in the last line we get the whole file hash like this:

    sha1: 97edb7d0d7daa7864c45edf14add33ec23ae94f8
