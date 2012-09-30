Connection Manager
==================

Do Not Use
==========

This is not done at all.

This is what I wrote to learn Go. It is a package that allows goroutines
to pass messages over channels, identifying the recipient by ID (or
broadcasting).

It kinda works, but not well.

It is posted here because I have seen a number of strange
exceptions^H^H^H^H^H^H errors and panics and I was encouraged by a
couple other Go enthusiasts to share it since the errors can be repro'd.

Errors I have seen:

    unexpected fault address 0x901f000a
    throw: fault
    [signal 0xb code=0x1 addr=0x901f000a pc=0x40a830]

    throw: invalid free

    panic: invalid memory address or nil pointer dereference
    throw: panic during gc
    [signal 0xb code=0x1 addr=0x1 pc=0x40a8e8]

The problem has a racy feel. Most runs are OK (until my system runs out
of socket descriptors!)  But the runs that crash, they crash immediately
when I start the rabbits.  I haven't seen it crash starting under 200
rabbits. (See below.)

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

errors/: dumps of some errors I have seen


Install
-------

You'll need go-uuid: http://code.google.com/p/go-uuid/

In examples, note there is a webroot directory.

Edit chat.go and change WEBROOT to the absolute path to that webroot
directory. Search for 8080 and change the port if you want to use a
different one.

Install chat and rabbits.

Run
---

Run chat. In a browser (only tried chrome, but should work with any...
har) bring up localhost port 8080. More windows == more fun.


Repro Steps
-----------

1. Run chat (or chat 2> errorlog)
2. In another window, run rabbits
3. If chat does not crash immediately, ^C out of rabbits, ^C out of
chat, then goto 1.

It usually takes me less than 10 tries to get one of those crashes.

Also, rabbits sucks up all the connections pretty fast and starts giving
Dial errors after a little bit. I'm not sure how to get around that, but
my gut says that's not related to the crashes.

Main in rabbits can be edited to control how many there are and how
often they chat.


My Setup
--------

Arch Linux, go 1.0.3, 4 GB, AMD Phenom(tm) II X4 965 Processor quadcore

chat and rabbits are coded to use NumCPUs threads.
