#!/usr/bin/python3

import atexit
import http.server
import os
import socketserver
import sys

class ImagesHandler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        # Only allow access to the local image server contents
        super().__init__(*args, directory="./local-image-server/", **kwargs)

    def do_GET(self):
        if not self.path.startswith("/os/"):
            super().send_error(404, "Not Found")
        else:
            self.path = self.path.removeprefix("/os")
            super().do_GET()

    def log_message(self, format, *args):
        pass

if len(sys.argv) != 2:
    print("Usage: " + sys.argv[0] + " <server ip>")
    exit(1)

script_dir = os.path.dirname(os.path.realpath(__file__))
pid_file = script_dir + "/local-images-server.pid"

def cleanup_pid():
    os.remove(pid_file)

# Only allow one instance to run at a time
if os.path.exists(pid_file):
    exit(0)

# Record the script's pid
with open(pid_file, "w") as f:
    f.write(str(os.getpid()))

# Remove pid file on exit
atexit.register(cleanup_pid)

with socketserver.TCPServer((sys.argv[1], 8123), ImagesHandler) as httpd:
    # Run until script is killed
    httpd.serve_forever()
