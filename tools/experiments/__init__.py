"""Experiment runners. Each module exposes an `async run()` coroutine
that registers a cast of bots against a live engine, optionally starts
the narrator as a subprocess, runs for a duration, and writes a
metrics summary to disk.
"""
