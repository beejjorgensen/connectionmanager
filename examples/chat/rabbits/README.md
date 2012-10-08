Rabbits
=======
This program spawns a number of connections to the chat.go server to
test loads.

Usage
-----
    usage ./rabbits [options]
    -c  (= 20)
        number of simultaneous bots to run
    -n  (= 1500)
        minimum delay time between chats (ms)
    -x  (= 4000)
        maximum delay time between chats (ms)

Bugs
----
If you hammer away too much, you're likely to run out of local
connections.

