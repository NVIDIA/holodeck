#!/bin/bash -e

# Install mdl
gem install mdl -v 0.13.0
# Run verify steps.
#
# Excluded paths:
#   docs/vendor       — vendored third-party content.
#   docs/superpowers  — internal AI-assisted planning artifacts (specs and
#                       implementation plans). These are work-in-progress
#                       authoring docs, not user-facing documentation, and
#                       are not held to mdl style rules.
find docs/ \( -path docs/vendor -o -path docs/superpowers \) -prune -false -o -name '*.md' -print | xargs mdl -s docs/mdl-style.rb
