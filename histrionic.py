# -*- coding: utf-8 -*-
"""Implements JSON version of xonsh histrionic backend."""

def install():
    import os.path
    import sys
    xonsh_ext_dir = os.path.expanduser('~/.xonsh')
    if os.path.isdir(xonsh_ext_dir):
        sys.path.append(xonsh_ext_dir)

    #$XONSH_HISTORY_BACKEND = Histrionic


import collections
import os
from xonsh.history.base import History, HistoryEntry


import json
import json_stream


# class HistoryEntry(types.SimpleNamespace):
#     """Represent a command in history.

#     Attributes
#     ----------
#     cmd: str
#         The command as typed by the user, including newlines
#     out: str
#         The output of the command, if xonsh is configured to save it
#     rtn: int
#         The return of the command (ie, 0 on success)
#     ts: two-tuple of floats
#         The timestamps of when the command started and finished, including
#         fractions.
#     cwd: str
#         The current working directory before execution the command.
#     """


class Histrionic(History):
    """Xonsh history backend base class.

    History objects should be created via a subclass of History.

    History acts like a sequence that can be indexed to return
    ``HistoryEntry`` objects.

    Note that the most recent command is the last item in history.

    Attributes
    ----------
    rtns : sequence of ints
        The return of the command (ie, 0 on success)
    inps : sequence of strings
        The command as typed by the user, including newlines
    tss : sequence of two-tuples of floats
        The timestamps of when the command started and finished, including
        fractions
    outs : sequence of strings
        The output of the command, if xonsh is configured to save it
    gc : A garbage collector or None
        The garbage collector

    In all of these sequences, index 0 is the oldest and -1 (the last item)
    is the newest.
    """

    def __init__(self, sessionid=None, filename=None, **kwargs):
        super().__init__(**kwargs)
        self._items = collections.OrderedDict()
        self._log = []

        if filename is None:
            filename = os.path.expanduser('~/.xonsh-history-debug.json')
        self.filename = filename
        try:
            with open(filename) as f:
                for cmd in json_stream.JSONStreamDecoder(f.read()):
                    self._append(cmd)
        except FileNotFoundError:
            pass
        self.file = open(filename, 'a')

        # Compatibility with bashisms and last prompt
        class InpView:
            def __init__(self, items):
                self.items = items
            def __len__(self):
                return len(self.items)
            def __getitem__(self, item):
#                print('inpview', self.items)
                return self.items[item]['inp']
        self.inps = InpView(self._log)

    def __len__(self):
        """Return the number of items in current session."""
        return len(self._items)

    def __getitem__(self, item):
        """Retrieve history entries, see ``History`` docs for more info."""
        if isinstance(item, int):
            if item >= len(self):
                raise IndexError("history index out of range")
            return HistoryEntry(**list(self._items.values())[item])
        elif isinstance(item, slice):
            cmds = self.inps[item]
            outs = self.outs[item]
            rtns = self.rtns[item]
            tss = self.tss[item]
            cwds = self.cwds[item]
            return [
                HistoryEntry(**x) for x in self._items[item]
            ]
        else:
            raise TypeError(
                "history indices must be integers "
                "or slices, not {}".format(type(item))
            )

    def _append(self, cmd):
        cmd['inp'] = cmd['inp'].rstrip()
        if cmd['inp'].startswith(' '):
            return
        self._log.append(cmd)
        self._items[cmd['inp']] = cmd
        self._items.move_to_end(cmd['inp'])

    def append(self, cmd):
        self._append(cmd)
        data = json.JSONEncoder().encode(cmd)
        self.file.write(data + '\n')

    def flush(self, **kwargs):
        self.file.flush()

    def items(self, newest_first=False):
        """Get history items of current session."""
        if newest_first:
            return reversed(self._items.values())
        else:
            return self._items.values()

    def all_items(self, newest_first=False):
        """Get all history items."""
        return self.items(newest_first)

    def info(self):
        """A collection of information about the shell history.

        Returns
        -------
        dict or collections.OrderedDict
            Contains history information as str key pairs.
        """
        raise NotImplementedError

    def run_gc(self, size=None, blocking=True):
        """Run the garbage collector.

        Parameters
        ----------
        size: None or tuple of a int and a string
            Determines the size and units of what would be allowed to remain.
        blocking: bool
            If set blocking, then wait until gc action finished.
        """
        pass

    def clear(self):
        """Clears the history of the current session from both the disk and
        memory.
        """
        pass
