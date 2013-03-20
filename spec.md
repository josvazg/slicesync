Spec
====

.slicesync files
----------------

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
