#!/usr/bin/env python3
"""fdprobe.py — exit 0 if process <pid> holds an open fd whose path contains <substr>.

Uses libproc's PROC_PIDLISTFDS / PROC_PIDFDVNODEPATHINFO so it costs ~1ms,
unlike `lsof -p` which takes hundreds of milliseconds per call.

Usage: fdprobe.py <pid> <path-substring>
"""
import ctypes
import ctypes.util
import sys

libproc = ctypes.CDLL(ctypes.util.find_library("proc"))
libc = ctypes.CDLL(ctypes.util.find_library("c"))

PROC_PIDLISTFDS = 1
PROC_PIDFDVNODEPATHINFO = 2
PROC_PIDLISTFD_SIZE = 8
PROC_PIDFDVNODEPATHINFO_SIZE = 2352  # sizeof(struct proc_fdvnodepathinfo) - CInfoPath offset check below
MAXPATHLEN = 1024


def main() -> int:
    pid, needle = int(sys.argv[1]), sys.argv[2].encode()
    bufsize = 4096 * PROC_PIDLISTFD_SIZE
    buf = (ctypes.c_char * bufsize)()
    n = libproc.proc_pidinfo(pid, PROC_PIDLISTFDS, 0, buf, bufsize)
    if n <= 0:
        return 1
    count = min(n // PROC_PIDLISTFD_SIZE, bufsize // PROC_PIDLISTFD_SIZE)
    ints = (ctypes.c_int * (count * 2)).from_buffer_copy(bytearray(buf[: count * 8]))
    fds = ints[0::2]  # struct proc_fdinfo { fd, fdtype }
    pathinfo = (ctypes.c_char * PROC_PIDFDVNODEPATHINFO_SIZE)()
    # struct proc_fdvnodepathinfo begins with two vnode structs whose path
    # (pvip.vip_path) starts at offset 8+... on all macOS releases the file
    # path lives in the second vip_path field; scan the whole buffer instead.
    for fd in fds:
        ctypes.memset(pathinfo, 0, PROC_PIDFDVNODEPATHINFO_SIZE)
        r = libproc.proc_pidfdinfo(pid, fd, PROC_PIDFDVNODEPATHINFO,
                                   pathinfo, PROC_PIDFDVNODEPATHINFO_SIZE)
        if r <= 0:
            continue
        if needle in bytes(pathinfo):
            return 0
    return 1


if __name__ == "__main__":
    sys.exit(main())
