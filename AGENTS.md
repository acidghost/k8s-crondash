# Instructions for Coding Agents

- build the app with `just build`
- dependencies are vendored, run `just vendor` to tidy and vendor
- dependencies are always locked and/or pinned to hashes
- before adding a dependency, always ask the user for the latest version
- after completing a task, run:
  - formatting: `just fmt`
  - linting: `just lint`

## Planning

- we almost always plan first, implement the plan later
- when asked to implement a plan, and the plan has a TODO list, ask the user if you should
  mark items as done and justify with validation proofs

## Knowledge Management

If you want to remember something, or you get corrected often about something, suggest the
user to update this document and suggest how to update it.
