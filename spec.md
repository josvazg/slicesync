# Spec

### SliceSync

SliceSync goal is to allow efficient big file versions updates from an standard HTTP server or the slicesync's syncserver itself.

To make it as fast and efficient as possible while not imposing too much load on the server side, slicesync prepares a hash dump (.slicesync) file for each file to be sent from the server. That file contains some information of the file to be downloaded, most importantly, the hash of each of the slices of that file and the full file hash. That info only needs to be re-calculated when the original file changes.


### Client

Slicesync client tool is given:

- A file URL to be downloaded
- An optional destination filename (it defaults to file URL's last path component name eg. 'a/b/c.txt' -> 'c.txt')
- An optional alike local file (it defaults to the destination file)
- An optional slice size (it defaults to the slice size in the downloaded .slicesync file)

The client tool then:

1. Downloads the remote .slicesync for the given URL (it must be pre-generated on the server)
2. Calculate the differences (wich may require to read or generate on the fly the local alike .slicesync to compare to)
4. Rebuild the remote file by mixing local available parts with remote parts
5. At the end the generated file hash is compared with the remote file hash on .slicesync


### Server

On the server side there are various options:

1. Use syncserver to serve the files over HTTP while (re)generating the hash dumps on the background.
2. Use any HTTP Range compliant server + start "shash -service" on the background.

In any case the files are served per directory or directory tree (recursively). Either syncserver or shash -service will generate a .slicesync directory at the base directory to be served. Within that .slicesync/ directory, .slicesync files will be populated for any new file that is copied to the managed directory or directory tree. Also, files deleted make the corresponding .slicesync file disappear.

Remember that, to keep the filesystem as clean as possible, all .slicesync files are placed within a single .slicesync directory, one per managed directory tree. Similar to what git or mercurial do with their .git or .hg directories.

Apart from that the server just needs to honour HTTP Range requests properly so that the client only gets the new parts not know before of the files to be downloaded. The server provides access to both the .slicesync files and the actual desired files.


#### Format of .slicesync files

Following zsync inspiration at http://zsync.moria.org.uk/paper/ch04s02.html, .slicesync files have the following header:

    Version: 1
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
