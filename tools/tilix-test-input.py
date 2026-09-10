"""Opt-in X11 input driver for uniquely named disposable Tilix test windows.

Never matches a normal user window. No global shortcuts or terminal settings.
"""
import ctypes as c
import sys
import time
import subprocess

title, action = sys.argv[1:3]
pid = int(sys.argv[3])
if not title.startswith('ssh-broadcast-live-'):
    raise SystemExit('Only disposable broadcast test windows are allowed')
if title not in subprocess.check_output(['ps', '-p', str(pid), '-o', 'args=']).decode():
    raise SystemExit('Process does not belong to this test')
x = c.CDLL('libX11.so.6')
xt = c.CDLL('libXtst.so.6')
display_t, window_t = c.c_void_p, c.c_ulong
class Attributes(c.Structure):
    _fields_ = [('x',c.c_int),('y',c.c_int),('width',c.c_int),('height',c.c_int),('border_width',c.c_int),('depth',c.c_int),('visual',c.c_void_p),('root',window_t),('window_class',c.c_int),('bit_gravity',c.c_int),('win_gravity',c.c_int),('backing_store',c.c_int),('backing_planes',c.c_ulong),('backing_pixel',c.c_ulong),('save_under',c.c_int),('colormap',c.c_ulong),('map_installed',c.c_int),('map_state',c.c_int),('all_event_masks',c.c_long),('your_event_mask',c.c_long),('do_not_propagate_mask',c.c_long),('override_redirect',c.c_int),('screen',c.c_void_p)]
x.XGetWindowAttributes.argtypes = [display_t,window_t,c.POINTER(Attributes)]
x.XOpenDisplay.argtypes = [c.c_char_p]; x.XOpenDisplay.restype = display_t
x.XDefaultRootWindow.argtypes = [display_t]; x.XDefaultRootWindow.restype = window_t
x.XQueryTree.argtypes = [display_t, window_t, c.POINTER(window_t), c.POINTER(window_t), c.POINTER(c.POINTER(window_t)), c.POINTER(c.c_uint)]
x.XFetchName.argtypes = [display_t, window_t, c.POINTER(c.c_void_p)]
x.XInternAtom.argtypes = [display_t, c.c_char_p, c.c_int]; x.XInternAtom.restype = c.c_ulong
x.XGetWindowProperty.argtypes = [display_t, window_t, c.c_ulong, c.c_long, c.c_long, c.c_int, c.c_ulong, c.POINTER(c.c_ulong), c.POINTER(c.c_int), c.POINTER(c.c_ulong), c.POINTER(c.c_ulong), c.POINTER(c.c_void_p)]
x.XFree.argtypes = [c.c_void_p]
x.XSetInputFocus.argtypes = [display_t, window_t, c.c_int, c.c_ulong]
x.XGetInputFocus.argtypes = [display_t, c.POINTER(window_t), c.POINTER(c.c_int)]
x.XRaiseWindow.argtypes = [display_t, window_t]
x.XDestroyWindow.argtypes = [display_t, window_t]
x.XFlush.argtypes = [display_t]
x.XStringToKeysym.argtypes = [c.c_char_p]; x.XStringToKeysym.restype = c.c_ulong
x.XKeysymToKeycode.argtypes = [display_t, c.c_ulong]; x.XKeysymToKeycode.restype = c.c_uint
xt.XTestFakeKeyEvent.argtypes = [display_t, c.c_uint, c.c_int, c.c_ulong]
d = x.XOpenDisplay(None)
if not d:
    raise SystemExit('No X11 display')

def find(w, depth=0):
    actual, fmt, count, after, data = c.c_ulong(), c.c_int(), c.c_ulong(), c.c_ulong(), c.c_void_p()
    x.XGetWindowProperty(d, w, x.XInternAtom(d, b'_NET_WM_PID', 0), 0, 1, 0, 0, c.byref(actual), c.byref(fmt), c.byref(count), c.byref(after), c.byref(data))
    if data.value:
        found = count.value and fmt.value == 32 and c.cast(data, c.POINTER(c.c_ulong))[0] == pid
        x.XFree(data)
        attrs = Attributes()
        if found and x.XGetWindowAttributes(d,w,c.byref(attrs)) and attrs.map_state == 2 and attrs.width > 100 and attrs.height > 100:
            return w
    name = c.c_void_p()
    if x.XFetchName(d, w, c.byref(name)) and name.value:
        value = c.string_at(name).decode(errors='replace'); x.XFree(name)
    if depth > 4:
        return None
    root, parent, children, count = window_t(), window_t(), c.POINTER(window_t)(), c.c_uint()
    if x.XQueryTree(d, w, c.byref(root), c.byref(parent), c.byref(children), c.byref(count)):
        ids = [children[i] for i in range(count.value)]
        if children: x.XFree(children)
        for child in ids:
            found = find(child, depth + 1)
            if found: return found
    return None

w = find(x.XDefaultRootWindow(d))
if not w:
    raise SystemExit('Exact disposable window not found: ' + title)
if action == '--close':
    x.XDestroyWindow(d, w); x.XFlush(d); raise SystemExit(0)
focus, revert = window_t(), c.c_int()
for attempt in range(5):
    x.XRaiseWindow(d, w); x.XSetInputFocus(d, w, 2, 0); x.XFlush(d)
    time.sleep(.2)
    x.XGetInputFocus(d, c.byref(focus), c.byref(revert))
    if focus.value == w: break
if focus.value != w:
    raise SystemExit('Test window did not receive focus; refusing to type')
for chord in action.split(','):
    keys = [x.XKeysymToKeycode(d, x.XStringToKeysym(k.encode())) for k in chord.split('+')]
    if not all(keys): raise SystemExit('Unknown test key')
    for key in keys: xt.XTestFakeKeyEvent(d, key, 1, 0)
    for key in reversed(keys): xt.XTestFakeKeyEvent(d, key, 0, 0)
    x.XFlush(d); time.sleep(.015)
