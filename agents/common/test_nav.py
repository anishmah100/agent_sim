from agents.common.nav import NavGrid

def g(rows): return NavGrid(len(rows[0]), len(rows), rows)

def test_straight_line():
    grid = g([".....", "....."])
    assert grid.next_dir((0,0),(4,0)) == "E"
    assert len(grid.astar((0,0),(4,0))) == 5

def test_wall_detour():
    rows = ["....", ".##.", ".##.", "...."]
    grid = g(rows)
    p = grid.astar((0,0),(3,0))
    assert p is not None and len(p) >= 4  # routed around the wall block

def test_unreachable_returns_none():
    rows = ["..#..", "..#..", "..#.."]  # solid wall column splits the map
    grid = g(rows)
    assert grid.astar((0,0),(4,0)) is None
    assert grid.next_dir((0,0),(4,0)) is None

def test_stop_adjacent_to_occupied_goal():
    grid = g(["....."])
    p = grid.astar((0,0),(4,0), dynamic_blocked=[(4,0)], stop_adjacent=True)
    assert p is not None and p[-1] == (3,0)  # ends NEXT to the occupied goal

def test_dynamic_blocker_detour():
    grid = g(["...", "...", "..."])
    p = grid.astar((0,0),(2,0), dynamic_blocked=[(1,0)])
    assert p is not None and (1,0) not in p  # went around the blocker

def test_off_grid_goal_none():
    grid = g(["..."])
    assert grid.astar((0,0),(9,9)) is None
