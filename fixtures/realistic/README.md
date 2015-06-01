## Realistic fixtures
#### JSON that tells a story

This directory contains valid pipeviz JSON messages. The intention is that, when sent to a pipeviz daemon in filename lexical order, they simulate a realistic series of messages that a real pipeviz instance might receive, in the case of someone setting up pipeviz around some existing software/infrastructure.

The files are named with a 3-digit leading pattern, followed by a brief description. The three-digit space ensures there’s copious space in which to interleave new messages later without needing to rename anything.

Being that JSON is rather averse to inline documentation, this file serves as a location to document the role each message plays in this story, in order.

In particular, the list also indicates the imagined client/source that would logically have access to all the data contained in the message. This client-speculation, along with the logical ordering of the messages themselves, are the two ways in which this set of fixtures aims to be “realistic.”

* **000-commits** (scan-repo) - since we’re imagining an existing git repository for the app that is our focal point, this is the most logical place to begin: a dump of all commits in the repository. (pipeviz’ own commit history is used here)
* **004-labels** (scan-repo) - follows pretty directly on the heels of 000-commits; this identifies the labels (branches and tags) that are present in the repository (at least, in the origin repo - branches still need to be namespaced). These are extracted just by scanning a repo.
* **007-test-stats** (scan-github) - scanner that slurps data from github to reports test pass/failure status as recorded by, say, travis-ci.
* **010-prod** (pvc) - manual reporting about the existence of all three of the servers involved in constituting “prod” - `prod-web01`, `prod-web02`, and `prod-db01.`
* **015-prod1-pkg-ls** (scan-yum) - a yum-based scanner, reports apache httpd binary and libphp5.so library on prod-web01.
* **020-prod1-app** (scan-drush) - imagining that the app is a Drupal instance, this would probably be a drush-based scanner, or at least something complemented by drush (it reports db connection details that would otherwise be prohibitively difficult to obtain). But this is more like other logic-state reporting messages than not.
* **025-prod1-procs** (scan-procs) - a scanner that inspects the process table and reports “useful” info; tells us about the running httpd proc. Particularly tricky but crucial here is getting the ‘logic-state’ associations right; getting the port listeners right is also essential.
* **030-prod2-pkg-ls** (scan-yum) - a yum-based scanner, reports apache httpd binary and libphp5.so library on prod-web02. Identical to the prod1 message except for env.
* **035-prod1-app** (scan-drush) - imagining that the app is a Drupal instance, this would probably be a drush-based scanner, or at least something complemented by drush (it reports db connection details that would otherwise be prohibitively difficult to obtain). But this is more like other logic-state reporting messages than not. Identical to the prod1 message except for env.
* **040-prod1-procs** (scan-procs) - a scanner that inspects the process table and reports “useful” info; tells us about the running httpd proc. Particularly tricky but crucial here is getting the ‘logic-state’ associations right; getting the port listeners right is also essential. Identical to the prod1 message except for env.
* **045-proddb1-pkg-ls** (scan-yum) - yum-based scanner reports mysql pkg.
* **050-proddb1-mysql** (scan-mysql) - a multifaceted scanner that both looks at the proc table, inspects a mysql config, and issues some queries to report on both the mysql process and the datasets contained therein.
