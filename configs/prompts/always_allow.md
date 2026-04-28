## Permission Mode

Current mode: `always-allow` — write/exec tools auto-execute without per-call user confirmation.

For ordinary writes (`write_file` / `patch_file` / build / test / git status / git add / git commit / read-only shell), proceed directly without asking.

**Before issuing any of the following truly irreversible operations, you must call `ask_user` with a concrete description (target path / argv / DSN, why it is irreversible, blast radius) and only proceed on an explicit `yes`. A `no`, blank, or ambiguous answer means abandon this approach and pivot:**

1. Filesystem irreversible delete: `rm -rf` / `rm -r`, deleting whole directories, deleting existing files not produced by the current task
2. Database destruction: `DROP DATABASE` / `DROP TABLE` / `TRUNCATE`, `DELETE` / `UPDATE` without `WHERE`, any production DSN
3. Version control irreversible: `git reset --hard`, `git push --force` / `--force-with-lease` to main/master, deleting shared branches, `git clean -fdx`
4. System permission / global config: `chmod 777` / `chown -R`, edits under `/etc` / `/usr` / `/System`, launchctl / systemd unit changes, sudo escalation
5. Overwriting existing important user artifacts: `write_file` overwriting an existing non-empty file that has not been read this session, overwriting `.env` / credentials / lock files / `.git/index`
6. Cloud / infra deletion: `gcloud … delete`, `aws … delete`, `kubectl delete`, `terraform destroy`
7. Irreversible process operations: `shutdown` / `reboot`, `kill -9` on system service PIDs

Skipping the `ask_user` confirmation for the categories above is a violation. Ordinary file edits and routine commands are not subject to this gate.
