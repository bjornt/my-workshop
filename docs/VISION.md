# Vision

## Product promise

`my-workshop` makes [Workshop](https://ubuntu.com/workshop) practical for
day-to-day development with the tools and workflow the developer chooses.
It fills gaps between a project's Workshop setup and an individual's needs,
whether or not the project already uses Workshop.

This document describes long-term direction, not a list of implemented
features. See the [project documentation](README.md) for current behavior.

## Primary use case

Run AI coding agents inside a Workshop environment so they can work with
less supervision while keeping their activity isolated from the host.
A project's SDK choices should not determine which coding harness a
developer can use, and the absence of a project Workshop configuration
should not prevent this workflow.

## Desired experience

### Bring your own SDKs

Developers can add personal SDKs, including their preferred coding harness,
to an existing Workshop setup or to one created for a project that does
not use Workshop. Personal choices do not require changes to the project's
shared configuration.

### Start with tools ready to use

Coding harnesses should be configured and ready when the environment starts,
without repeated authentication or manual setup for every new Workshop.
For example, a developer can seed an SDK's persistent configuration directory
from their existing configuration.

### Connect explicitly chosen host services automatically

When a developer explicitly chooses a personal SDK and a host-to-container
port connection, starting the environment should establish that connection
without a second confirmation command.

That consent does not extend to arbitrary host-service connections declared
by a project. Exposing a host service to the container is a security decision.
One intended use is making the oh-my-pi authentication gateway available
inside the environment without giving the coding agent the real API keys.

## Principles

- **Developer choice:** complement the project's setup with the developer's
  preferred tools and workflow.
- **Low friction:** automate repeated setup without getting in the way.
- **Works with or without Workshop configuration:** support existing Workshop
  projects and projects that have not adopted it.
- **Protect the Git working tree:** keep personal additions and generated or
  modified Workshop configuration out of accidental commits.
- **Isolation with deliberate access:** use Workshop to isolate agent activity;
  access to host files, credentials, and services should reflect the
  developer's choices. Isolation is the goal, not a promise of zero risk.

## Boundaries and long-term direction

`my-workshop` complements Workshop; it does not replace Workshop's environment
runtime or require projects to adopt a particular coding harness.

The aim is to close workflow gaps, not to maintain a competing platform.
Ideally, these capabilities will become part of Workshop itself, reducing
the need for this wrapper.
