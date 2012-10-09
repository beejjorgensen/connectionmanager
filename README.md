Connection Manager
==================

Do Not Use
==========
This is not done at all.

This is what I wrote to learn Go. It is a package that allows goroutines
to pass messages over channels, identifying the recipient by ID (or
broadcasting).

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

Install and Run
---------------
See the README in the examples/chat directory.

TODO
----
* Get rid of ConnectRequest? Just add new UIDs when events happen?
* Add timeout to eliminate old connections
* Throw away old messages if list gets large
* Helper functions for SendMessage?
* Allow user to turn polling off explicitly
* Unicast message
* Multicast message
* Multicast groups (pub/sub-style)

Bugs
----
All kinds, I'm sure.

Rabbits sucks up all the connections pretty fast and starts giving
Dial errors after a little bit. I'm not sure how to get around that, but
my gut says that's not related to the crashes.

