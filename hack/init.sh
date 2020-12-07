#!/usr/bin/env sh

if [[ "$1" == "" ]]; then
  echo "please give a appropriate name"
  exit 1
fi

find . -name "*.yaml" -exec sed -i '' s/github-go/"$1"/ {} +
find . -name "*.yml" -exec sed -i '' s/github-go/"$1"/ {} +
find . -name "*.md" -exec sed -i '' s/github-go/"$1"/ {} +
