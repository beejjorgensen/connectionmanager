Connection Manager
==================

Do Not Use
==========
This is not done at all.

This is what I wrote to learn Go. It is a package that allows goroutines
to pass messages over channels, identifying the recipient by ID (or
broadcasting).

It kinda works, but not well.

I originally posted this because I was getting weird low-level crashes.
It was due to multiple goroutines accessing the same map without a lock.
Thanks, go-nuts!

Code Notes
----------
Since I'm a Go beginner, if anyone has feedback in general, I'll take
it.

I purposefully didn't optimize, here. I figured I'd just use a pile
of language features and see how it held up.

Files
-----
connectionmanager.go: the package file

examples/chat.go: a sample long-poll chat server that uses a
connectionmanager.

examples/webchat: HTML/JS page for talking to the chat server

examples/rabbits/rabbits.go: bot chatters for hammering the server

Install
-------
In examples, note there is a webroot directory.

Edit chat.go and change WEBROOT to the absolute path to that webroot
directory. Search for 8080 and change the port if you want to use a
different one.

Install chat and rabbits.

Run
---
Run chat. In a browser (only tried chrome, but should work with any...
har) bring up localhost port 8080. More windows == more fun.


rabbits starts a bunch of bot connections. It can be tuned in its main.

Bugs
----
All kinds, I'm sure.

Rabbits sucks up all the connections pretty fast and starts giving
Dial errors after a little bit. I'm not sure how to get around that, but
my gut says that's not related to the crashes.

