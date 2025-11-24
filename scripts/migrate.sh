#!/bin/sh

atlas schema apply \
	--url $DATABASE_URL \
	--to "file://atlas/schema.hcl" \
	"$@"
