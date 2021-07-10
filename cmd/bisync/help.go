package bisync

import (
	"strconv"
	"strings"
)

func makeHelp(help string) string {
	replacer := strings.NewReplacer(
		"|", "`",
		"{MAXDELETE}", strconv.Itoa(DefaultMaxDelete),
		"{CHECKFILE}", DefaultCheckFilename,
		"{WORKDIR}", DefaultWorkdir,
	)
	return replacer.Replace(help)
}

var shortHelp string = `Perform bidirectonal synchronization between two paths.`

var rcHelp string = makeHelp(`This takes the following parameters

- path1 - a remote directory string e.g. |drive:path1|
- path2 - a remote directory string e.g. |drive:path2|
- dryRun - dry-run mode
- resync - performs the resync run
- checkAccess - abort if {CHECKFILE} files are not found on both filesystems
- checkFilename - file name for checkAccess (default: {CHECKFILE})
- maxDelete - abort sync if percentage of deleted files is above
  this threshold (default: {MAXDELETE})
- force - maxDelete safety check and run the sync
- checkSync - |true| by default, |false| disables comparison of final listings,
              |only| will skip sync, only compare listings from the last run
- removeEmptyDirs - remove empty directories at the final cleanup step
- filtersFile - read filtering patterns from a file
- workdir - server directory for history files (default: {WORKDIR})
- noCleanup - retain working files

See the [bisync](https://rclone.org/commands/rclone_bisync/) command
for more information on the above.`)

var longHelp string = shortHelp + makeHelp(`

[Bisync](https://rclone.org/bisync/) provides a
bidirectional cloud sync solution in rclone.
It retains the Path1 and Path2 filesystem listings from the prior run.
On each successive run it will:
- list files on Path1 and Path2, and check for changes on each side.
  Changes include |New|, |Newer|, |Older|, and |Deleted| files.
- Propagate changes on Path1 to Path2, and vice-versa.

### Safety measures
- Lock file prevents multiple simultaneous runs when taking a while.
- Handle change conflicts non-destructively by creating
  |..path1| and |..path2| file versions.
- File system access health check using |{CHECKFILE}| files
  (see the |--check-access| flag).
- Abort on excessive deletes - protects against a failed listing
  being interpreted as all the files were deleted.
  See the |--max-delete| and |--force| flags.
- If something evil happens, bisync goes into a safe state to block
  damage by later runs.
  See [Error handling](https://rclone.org/bisync/#error-handling).

### Modification time
Bisync relies on file timestamps to identify changed files and will
_refuse_ to operate if backend lacks the modification time support.

If you or your application should change the content of a file
without changing the modification time then bisync will _not_
notice the change, and thus will not copy it to the other side.

Note that on some cloud storage systems it is not possible to have file
timestamps that match _precisely_ between the local and other filesystems.
Bisync's approach to this problem is by tracking the Path1-to-Path1 and
Path2-to-Path2 deltas on each side _separately_, and then applying
the resulting changes on the other side.

### Error Handling
Certain bisync critical errors, such as file copy/move failing, will result in
a bisync lockout of following runs. The lockout is asserted because the sync
status and history of the Path1 and Path2 filesystems cannot be trusted,
so it is safer to block any further changes until someone checks things out.
The recovery is to do a |--resync| again.

It is recommended to use |--resync --dry-run --verbose| initially and
_carefully_ review what changes will be made before running the |--resync|
without |--dry-run|.

Most of these events come up due to a error status from an internal call.
On such a critical error the |{...}.path1.lst| and |{...}.path2.lst|
listing files are renamed to extension |.lst-err|, which blocks any future
bisync runs (since the normal |.lst| files are not found).

Some errors are considered temporary and re-running the bisync is not blocked.
The _critical return_ blocks further bisync runs.

### Command line syntax
Path1 and Path2 arguments may be references to any mix of local directory
paths (absolute or relative), UNC paths (|//server/share/path|),
Windows drive paths (with a drive letter and |:|) or configured remotes
with optional subdirectory paths. Cloud references are distinguished by
having a |:| in the argument
(see [Windows support](https://rclone.org/bisync/#windows)).

Path1 and Path2 are treated equally, in that neither has priority for
file changes, and access efficiency does not change whether a remote
is on Path1 or Path2.

The listings in bisync working directory (default: |{WORKDIR}|)
are named based on the Path1 and Path2 arguments so that separate syncs
to individual directories within the tree may be set up, e.g.:
|path_to_local_tree..dropbox_subdir.lst|.

Arbitrary rclone flags may be specified on the bisync command line.
The most important bisync flags are listed below.

#### --resync
This will effectively make both Path1 and Path2 filesystems contain a
matching superset of all files. Path2 files that do not exist in Path1 will
be copied to Path1, and the process will then sync the Path1 tree to Path2.

The base directories on both the Path1 and Path2 filesystems must exist
or bisync will fail. This is required for safety - that bisync can verify
that both paths are valid.

When using |--resync| a newer version of a file on the Path2 filesystem
will be overwritten by the Path1 filesystem version.
Carefully evaluate deltas using |--dry-run|.

For a resync run, one of the paths may be empty (no files in the path tree).
The resync run should result in files on both paths, else a normal non-resync
run will fail.

For a non-resync run, either path being empty (no files in the tree) fails with
|Empty current PathN listing. Cannot sync to an empty directory: X.pathN.lst|
This is a safety check that an unexpected empty path does not result in
deleting **everything** in the other path.

#### --check-access
Access check files are an additional safety measure against data loss.
bisync will ensure it can find matching |{CHECKFILE}| files in the same places
in the Path1 and Path2 filesystems.
Time stamps and file contents are not important, just the names and locations.
Place one or more |{CHECKFILE}| files in the Path1 or Path2 filesystem and
then do either a run without |--check-access| or a |--resync| to set
matching files on both filesystems.
If you have symbolic links in your sync tree it is recommended to place
|{CHECKFILE}| files in the linked-to directory tree to protect against
bisync assuming a bunch of deleted files if the linked-to tree should not be
accessible. Also see the |--check-filename| flag.

#### --max-delete
As a safety check, if greater than the |--max-delete| percent of files were
deleted on either the Path1 or Path2 filesystem, then bisync will abort with
a warning message, without making any changes.
The default |--max-delete| is |{MAXDELETE}%|.
One way to trigger this limit is to rename a directory that contains more
than half of your files. This will appear to bisync as a bunch of deleted
files and a bunch of new files.
This safety check is intended to block bisync from deleting all of the
files on both filesystems due to a temporary network access issue, or if
the user had inadvertently deleted the files on one side or the other.
To force the sync either set a different delete percentage limit,
e.g. |--max-delete 75| (allows up to 75% deletion), or use |--force|
to bypass the check.

Also see the [all files changed](https://rclone.org/bisync/#all-files-changed) check.

#### --filters-file
By using rclone filter features you can exclude file types or directory
sub-trees from the sync.
See the [Bisync filters](https://rclone.org/bisync/#filtering) and generic
[--filter-from](https://rclone.org/filtering/#filter-from-read-filtering-patterns-from-a-file)
documentation.

Bisync calculates an MD5 hash of the filters file
and stores the hash in a |.md5| file in the same place as your filters file.
On the next runs with |--filters-file| set, bisync re-calculates the MD5 hash
of the current filters file and compares it to the hash stored in |.md5| file.
If they don't match the run aborts with a critical error and thus forces you
to do a |--resync|, likely avoiding a disaster.

#### --check-sync
Enabled by default, the check-sync function checks that all of the same
files exist in both the Path1 and Path2 history listings. This _check-sync_
integrity check is performed at the end of the sync run by default.
Any untrapped failing copy/deletes between the two paths might result
in differences between the two listings and in the untracked file content
differences between the two paths. A resync run would correct the error.

Note that the default-enabled integrity check locally executes a load of both
the final Path1 and Path2 listings, and thus adds to the run time of a sync.
Using |--check-sync=false| will disable it and may significantly reduce the
sync run times for very large numbers of files.

The check may be run manually with |--check-sync=only|. It runs only the
integrity check and terminates without actually synching.
`)
