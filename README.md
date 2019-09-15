# Gocrypt SFTP

Gocrypt SFTP is a SFTP proxy server that aims to greatly improve the performance
of bidirectionally syncing files encrypted with
[gocryptfs](https://github.com/rfjakob/gocryptfs) to a remote (and potentially
untrusted) server.

## Usage

A config JSON file is required. All displayed fields must be filled out:

```json
{
  "ProxyUser": "proxy-user-name",
  "ProxyPassword": "proxy-password",
  "KnownHostsPath": "/path/to/homedir/.ssh/known_hosts",
  "Remote": {
    "Addr": "remote.server.com:22",
    "FileRoot": "/remote/path/to/gocryptfs/drive/",
    "User": "remote-user-name",
    "PrivateKeyPath": "/path/to/homedir/.ssh/id_rsa"
  }
}
```

Gocrypt SFTP can be run with:

`./gocryptsftp -c config.json`

When starting up, Gocrypt SFTP will ask for passphrases as needed.

You can then connect to the SFTP proxy server by connecting to
`localhost:9022` with the configured proxy user and proxy password.

Gocrypt SFTP will create connections as needed. to the `Remote.Addr` with the
provided `Remote.User` and `Remote.PrivateKeyPath`.

## Experimental

This tool is still in the experimental stage, so only a limited feature set is
supported. For instance, Gocrypt SFTP does not support long file names, and it
assumes filename encryption is turned on (if it were turned off you don't need
a proxy at all).

## Motivation
There are existing ways to do syncing with a remote server with files
encrypted by gocryptfs, but they are all not ideal solutions.

1. Mounting a remote FS with SSHFS or using CIFS, then mounting a gocryptfs
drive on top of that. One then syncs a local directory directly to this
nested drive.
  - Doing this has proved an extremely slow when determining scanning files
  (it takes over 10 minutes to stat 40,000 files, and does not max out CPU
  or network IO).
  - One major reason for this is probably a poor use of multithreading. While
  gocryptfs may have good support for it, the underlying FS might not. For
  instance, getting high concurrency to work on CIFS is either impossible or
  really hard to do. Also existing SSHFS solutions on Windows don't seem too
  mature in this regard. Also I can never seem to max out my network bandwidth
  with FUSE on Windows for some reason.
  - I haven't proven this, but I believe part of the reason has something to do
  with poor caching of directory IVs, which is made worse by having to fetch
  the file from the network again. Perhaps gocryptfs's performance relies on
  the underlying FS being a local drive.

2. Creating a separate gocryptfs drive locally. One then syncs a local
directory to this local gocryptfs drive, and then syncs this gocryptfs drive
to a remote server.
  - Doing this is relatively efficient compared to the first solution.
  (On my laptop with Intel Core i7-8565U, with about 20,000 files, it takes
  about 20 seconds to stat all files. Then syncing to to a remote server with
  SFTP takes about 40 seconds to stat all files).
  - Huge plus in this regard is no longer having to deal with FUSE on Windows.
  - Major disadvantage to this approach is one must have enough storage for
  both the entire gocryptfs drive and the plaintext files that we actually want.
  Basically it defeats the purpose of selective syncing.

## Design

Gocrypt SFTP is designed to access the remote encrypted drive as efficiently as
possible by introducing a few simple caching mechanisms.

For instance, if file name encryption is enabled, gocryptfs requires a
`gocrypt.diriv` in every directory to encrypt/decrypt file names in that
directory. Making concurrent requests to the drive would normally result in
multiple requests for the same `gocrypt.diriv` files, especially when
requesting for files in the same directories. Gocrypt SFTP is designed to
prevent duplicating this work and reuse cached directory IVs whenever
possible.

Gocrypt SFTP also applies the same optimization for listing files in
a directory and decryption of names.

Another possible source of inefficiencies is trying to find a certain file in
a given directory. Normally, this would require linearly walking through
encrypted names in a directory, decrypting them and testing the plaintext
name for a match. To make this process more efficient, Gocrypt SFTP
introduces a simple LRU path lookup cache for frequently accessed paths.

With these simple cache mechanisms in place, Gocrypt SFTP outperforms as well
as or even better than the second approach described in
[Motivations](#motivations). On my laptop with Intel Core i7-8565U, with
about 20,000 files, it takes under 1 minute to stat (in plaintext) all files
in the remote gocryptfs drive.




