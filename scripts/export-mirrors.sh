#!/bin/bash

export REPOS="HumorChecker dynamolock errors goherokuname runner supervisor bookmarkd snippetsd cci mlflappygopher kohrah-ani oversight pglock junk"

for repo in $REPOS
do
	./scripts/push-repo.sh $repo
done

