# 優化進度

Agenvoy 是一人開發的專案，從 2 月到 10 月幾乎每天不間斷地開發，一路以推進功能為主，很少回頭整理已經寫好的程式碼。
累積到現在，整體的細節已經對個人記憶有點壓力，為了往後方便維護，也是時候放慢腳步開始整理。

並且這一年開始深入研究 Rust，一些過往的壞習慣與觀念需要調整，所以年初與現在的寫法已經有落差。
因此這段時間以檢查與整理為主：會逐檔看過一次，清掉 dead code 與優化實作，並把錯誤訊息、命名與寫法統一到現在的習慣。

同時也會開始盡量補上註解，讓程式碼從「只有我看得懂」轉向方便他人閱讀。

## 檔案（進度 4.7%）

### `internal/agents/audio`

- [x] `internal/agents/audio/audio.go`
- [x] `internal/agents/audio/speak.go`
- [x] `internal/agents/audio/transcribe.go`

### `internal/agents/decision`

- [ ] `internal/agents/decision/decision.go`
- [ ] `internal/agents/decision/relevance.go`

### `internal/agents/exec/allow/skill`

- [x] `internal/agents/exec/allow/skill/match.go`
- [x] `internal/agents/exec/allow/skill/toggle.go`

### `internal/agents/exec/allow/tool`

- [x] `internal/agents/exec/allow/tool/match.go`
- [x] `internal/agents/exec/allow/tool/parse.go`
- [x] `internal/agents/exec/allow/tool/toggle.go`

### `internal/agents/exec`

- [ ] `internal/agents/exec/assign_skill.go`
- [ ] `internal/agents/exec/assign_turn_context.go`
- [ ] `internal/agents/exec/cancel.go`
- [ ] `internal/agents/exec/execute.go`
- [ ] `internal/agents/exec/irreversible.go`
- [ ] `internal/agents/exec/push.go`
- [ ] `internal/agents/exec/reset.go`
- [ ] `internal/agents/exec/response.go`
- [ ] `internal/agents/exec/run.go`
- [ ] `internal/agents/exec/saveHistory.go`
- [ ] `internal/agents/exec/schedule.go`
- [ ] `internal/agents/exec/sendError.go`
- [ ] `internal/agents/exec/sendEvent.go`
- [ ] `internal/agents/exec/session.go`
- [ ] `internal/agents/exec/skill_filter.go`
- [ ] `internal/agents/exec/steerWait.go`
- [ ] `internal/agents/exec/stream.go`
- [ ] `internal/agents/exec/streamText.go`
- [ ] `internal/agents/exec/summary_filter.go`
- [ ] `internal/agents/exec/systemInfo.go`
- [ ] `internal/agents/exec/systemPrompt.go`
- [ ] `internal/agents/exec/toolCall.go`
- [ ] `internal/agents/exec/tool_gap.go`

### `internal/agents/exec/chatbot`

- [ ] `internal/agents/exec/chatbot/admin.go`

### `internal/agents/exec/compact`

- [ ] `internal/agents/exec/compact/assembleMessages.go`
- [ ] `internal/agents/exec/compact/checkThreshold.go`
- [ ] `internal/agents/exec/compact/copilotLimit.go`
- [ ] `internal/agents/exec/compact/extractContextHistory.go`
- [ ] `internal/agents/exec/compact/modelLimit.go`
- [ ] `internal/agents/exec/compact/rawToolFallback.go`
- [ ] `internal/agents/exec/compact/sessionHistory.go`
- [ ] `internal/agents/exec/compact/toolHistory.go`
- [ ] `internal/agents/exec/compact/trimDecision.go`
- [ ] `internal/agents/exec/compact/trimFallback.go`

### `internal/agents/exec/explain`

- [ ] `internal/agents/exec/explain/explain.go`

### `internal/agents/exec/fast`

- [ ] `internal/agents/exec/fast/fast.go`

### `internal/agents/exec/followup`

- [ ] `internal/agents/exec/followup/followup.go`

### `internal/agents/exec/guide`

- [ ] `internal/agents/exec/guide/guide.go`

### `internal/agents/exec/memory`

- [ ] `internal/agents/exec/memory/save.go`
- [ ] `internal/agents/exec/memory/search.go`

### `internal/agents/exec/model`

- [ ] `internal/agents/exec/model/check_alive.go`
- [ ] `internal/agents/exec/model/list.go`
- [ ] `internal/agents/exec/model/next.go`
- [ ] `internal/agents/exec/model/route.go`
- [ ] `internal/agents/exec/model/select.go`
- [ ] `internal/agents/exec/model/select_with_decision.go`
- [ ] `internal/agents/exec/model/select_with_llm.go`

### `internal/agents/exec/readonly`

- [ ] `internal/agents/exec/readonly/argv.go`
- [ ] `internal/agents/exec/readonly/shell.go`

### `internal/agents/exec/retryHandler`

- [ ] `internal/agents/exec/retryHandler/reason.go`
- [ ] `internal/agents/exec/retryHandler/retryHandler.go`

### `internal/agents/exec/subagent`

- [ ] `internal/agents/exec/subagent/backgroundLog.go`
- [ ] `internal/agents/exec/subagent/ensureSession.go`
- [ ] `internal/agents/exec/subagent/exec.go`
- [ ] `internal/agents/exec/subagent/sendEvent.go`
- [ ] `internal/agents/exec/subagent/waitResume.go`

### `internal/agents/exec/summary`

- [ ] `internal/agents/exec/summary/decision.go`
- [ ] `internal/agents/exec/summary/generate.go`
- [ ] `internal/agents/exec/summary/send.go`

### `internal/agents/exec/todo`

- [ ] `internal/agents/exec/todo/writeTodo.go`

### `internal/agents`

- [ ] `internal/agents/host.go`

### `internal/agents/image`

- [x] `internal/agents/image/generate.go`
- [x] `internal/agents/image/provider.go`

### `internal/agents/keychain`

- [ ] `internal/agents/keychain/keychain.go`

### `internal/agents/probe`

- [ ] `internal/agents/probe/probe.go`

### `internal/agents/sudo`

- [ ] `internal/agents/sudo/password.go`

### `internal/agents/types`

- [ ] `internal/agents/types/agent.go`
- [ ] `internal/agents/types/event.go`
- [ ] `internal/agents/types/usage.go`

### `internal/app`

- [ ] `internal/app/agentRegistry.go`
- [ ] `internal/app/daemonLog.go`
- [ ] `internal/app/deliverBackground.go`
- [ ] `internal/app/generateSummary.go`
- [ ] `internal/app/loopbackListeners.go`
- [ ] `internal/app/newMCP.go`
- [ ] `internal/app/pushHook.go`
- [ ] `internal/app/reloadDiscord.go`
- [ ] `internal/app/reloadRetry.go`
- [ ] `internal/app/reloadTelegram.go`
- [ ] `internal/app/runSkill.go`
- [ ] `internal/app/spawnDaemon.go`
- [ ] `internal/app/watchConfig.go`
- [ ] `internal/app/watchSession.go`

### `internal/filesystem`

- [ ] `internal/filesystem/db.go`
- [ ] `internal/filesystem/media.go`
- [ ] `internal/filesystem/path.go`
- [ ] `internal/filesystem/pathRule.go`
- [ ] `internal/filesystem/reader.go`
- [ ] `internal/filesystem/replyLang.go`
- [ ] `internal/filesystem/runtime.go`

### `internal/filesystem/record`

- [ ] `internal/filesystem/record/download.go`
- [ ] `internal/filesystem/record/log.go`
- [ ] `internal/filesystem/record/storeTemp.go`

### `internal/filesystem/skill`

- [ ] `internal/filesystem/skill/scheduleSkill.go`
- [ ] `internal/filesystem/skill/skill.go`

### `internal/locale`

- [ ] `internal/locale/askUser.go`
- [ ] `internal/locale/challenge.go`
- [ ] `internal/locale/chatbotCode.go`
- [ ] `internal/locale/confirm.go`
- [ ] `internal/locale/locale.go`
- [ ] `internal/locale/refusal.go`

### `internal/runtime`

- [ ] `internal/runtime/cancelNotify.go`
- [ ] `internal/runtime/cron.go`
- [ ] `internal/runtime/listener.go`
- [ ] `internal/runtime/pending.go`
- [ ] `internal/runtime/resume.go`
- [ ] `internal/runtime/runtime.go`
- [ ] `internal/runtime/scheduler.go`
- [ ] `internal/runtime/skill.go`
- [ ] `internal/runtime/task.go`
- [ ] `internal/runtime/version.go`

### `internal/runtime/chatbot`

- [ ] `internal/runtime/chatbot/chatbot.go`
- [ ] `internal/runtime/chatbot/chunk.go`
- [ ] `internal/runtime/chatbot/command.go`
- [ ] `internal/runtime/chatbot/reply.go`
- [ ] `internal/runtime/chatbot/sanitize.go`

### `internal/runtime/chatbot/discord`

- [ ] `internal/runtime/chatbot/discord/attachments.go`
- [ ] `internal/runtime/chatbot/discord/new.go`
- [ ] `internal/runtime/chatbot/discord/pending.go`
- [ ] `internal/runtime/chatbot/discord/push.go`
- [ ] `internal/runtime/chatbot/discord/reply.go`
- [ ] `internal/runtime/chatbot/discord/resume.go`
- [ ] `internal/runtime/chatbot/discord/run.go`

### `internal/runtime/chatbot/telegram`

- [ ] `internal/runtime/chatbot/telegram/attachments.go`
- [ ] `internal/runtime/chatbot/telegram/fileMarker.go`
- [ ] `internal/runtime/chatbot/telegram/filegroup.go`
- [ ] `internal/runtime/chatbot/telegram/new.go`
- [ ] `internal/runtime/chatbot/telegram/pending.go`
- [ ] `internal/runtime/chatbot/telegram/push.go`
- [ ] `internal/runtime/chatbot/telegram/reply.go`
- [ ] `internal/runtime/chatbot/telegram/resume.go`
- [ ] `internal/runtime/chatbot/telegram/run.go`

### `internal/runtime/chatbot/tool`

- [ ] `internal/runtime/chatbot/tool/list.go`
- [ ] `internal/runtime/chatbot/tool/platform.go`
- [ ] `internal/runtime/chatbot/tool/record.go`
- [ ] `internal/runtime/chatbot/tool/register.go`
- [ ] `internal/runtime/chatbot/tool/send.go`

### `internal/runtime/daemon`

- [ ] `internal/runtime/daemon/client.go`

### `internal/runtime/ipc`

- [ ] `internal/runtime/ipc/askUser.go`
- [ ] `internal/runtime/ipc/client.go`
- [ ] `internal/runtime/ipc/frame.go`
- [ ] `internal/runtime/ipc/mcp.go`
- [ ] `internal/runtime/ipc/pending.go`
- [ ] `internal/runtime/ipc/run.go`
- [ ] `internal/runtime/ipc/server.go`

### `internal/runtime/mcp`

- [ ] `internal/runtime/mcp/client.go`
- [ ] `internal/runtime/mcp/config.go`
- [ ] `internal/runtime/mcp/expose.go`
- [ ] `internal/runtime/mcp/mcp.go`
- [ ] `internal/runtime/mcp/oauth.go`
- [ ] `internal/runtime/mcp/poll.go`
- [ ] `internal/runtime/mcp/register.go`
- [ ] `internal/runtime/mcp/server.go`
- [ ] `internal/runtime/mcp/watch.go`

### `internal/runtime/pubsub`

- [ ] `internal/runtime/pubsub/pubsub.go`
- [ ] `internal/runtime/pubsub/subscriber.go`

### `internal/runtime/routes/handler`

- [ ] `internal/runtime/routes/handler/allowlist.go`
- [ ] `internal/runtime/routes/handler/automation.go`
- [ ] `internal/runtime/routes/handler/cancel.go`
- [ ] `internal/runtime/routes/handler/channel.go`
- [ ] `internal/runtime/routes/handler/config.go`
- [ ] `internal/runtime/routes/handler/confirm.go`
- [ ] `internal/runtime/routes/handler/daemonLog.go`
- [ ] `internal/runtime/routes/handler/drainEvents.go`
- [ ] `internal/runtime/routes/handler/file.go`
- [ ] `internal/runtime/routes/handler/fileLocate.go`
- [ ] `internal/runtime/routes/handler/info.go`
- [ ] `internal/runtime/routes/handler/key.go`
- [ ] `internal/runtime/routes/handler/keys.go`
- [ ] `internal/runtime/routes/handler/mcpOAuth.go`
- [ ] `internal/runtime/routes/handler/mcpServers.go`
- [ ] `internal/runtime/routes/handler/memory.go`
- [ ] `internal/runtime/routes/handler/model.go`
- [ ] `internal/runtime/routes/handler/multilog.go`
- [ ] `internal/runtime/routes/handler/pending.go`
- [ ] `internal/runtime/routes/handler/providerQuota.go`
- [ ] `internal/runtime/routes/handler/providers.go`
- [ ] `internal/runtime/routes/handler/role.go`
- [ ] `internal/runtime/routes/handler/send.go`
- [ ] `internal/runtime/routes/handler/sendFollowup.go`
- [ ] `internal/runtime/routes/handler/sendResult.go`
- [ ] `internal/runtime/routes/handler/sendSSE.go`
- [ ] `internal/runtime/routes/handler/session.go`
- [ ] `internal/runtime/routes/handler/sessionArtifact.go`
- [ ] `internal/runtime/routes/handler/skill.go`
- [ ] `internal/runtime/routes/handler/systemUpdate.go`
- [ ] `internal/runtime/routes/handler/tools.go`
- [ ] `internal/runtime/routes/handler/toriiInspect.go`
- [ ] `internal/runtime/routes/handler/voice.go`
- [ ] `internal/runtime/routes/handler/workDir.go`

### `internal/runtime/routes/handler/chatCompletions`

- [ ] `internal/runtime/routes/handler/chatCompletions/chatCompletions.go`
- [ ] `internal/runtime/routes/handler/chatCompletions/run.go`
- [ ] `internal/runtime/routes/handler/chatCompletions/stream.go`

### `internal/runtime/routes`

- [ ] `internal/runtime/routes/new.go`
- [ ] `internal/runtime/routes/page.go`
- [ ] `internal/runtime/routes/sw.go`

### `internal/runtime/store`

- [ ] `internal/runtime/store/actionHistory.go`
- [ ] `internal/runtime/store/fileOrigin.go`
- [ ] `internal/runtime/store/fileQuery.go`
- [ ] `internal/runtime/store/fileRecord.go`
- [ ] `internal/runtime/store/fileRestore.go`
- [ ] `internal/runtime/store/fileStore.go`
- [ ] `internal/runtime/store/message.go`
- [ ] `internal/runtime/store/messageSearch.go`
- [ ] `internal/runtime/store/session.go`
- [ ] `internal/runtime/store/store.go`

### `internal/runtime/toolAdapter/script`

- [ ] `internal/runtime/toolAdapter/script/ececute.go`
- [ ] `internal/runtime/toolAdapter/script/translator.go`

### `internal/runtime/torii`

- [ ] `internal/runtime/torii/list.go`
- [ ] `internal/runtime/torii/session.go`
- [ ] `internal/runtime/torii/torii.go`

### `internal/runtime/tui`

- [ ] `internal/runtime/tui/cmdSelector.go`
- [ ] `internal/runtime/tui/commandAdminChannel.go`
- [ ] `internal/runtime/tui/commandAudioModel.go`
- [ ] `internal/runtime/tui/commandBackground.go`
- [ ] `internal/runtime/tui/commandBot.go`
- [ ] `internal/runtime/tui/commandChannel.go`
- [ ] `internal/runtime/tui/commandCompact.go`
- [ ] `internal/runtime/tui/commandConfig.go`
- [ ] `internal/runtime/tui/commandCron.go`
- [ ] `internal/runtime/tui/commandDecisionModel.go`
- [ ] `internal/runtime/tui/commandDiscord.go`
- [ ] `internal/runtime/tui/commandDispatcher.go`
- [ ] `internal/runtime/tui/commandImageModel.go`
- [ ] `internal/runtime/tui/commandKey.go`
- [ ] `internal/runtime/tui/commandLog.go`
- [ ] `internal/runtime/tui/commandMcp.go`
- [ ] `internal/runtime/tui/commandMcpAdd.go`
- [ ] `internal/runtime/tui/commandMcpLogin.go`
- [ ] `internal/runtime/tui/commandMcpPermission.go`
- [ ] `internal/runtime/tui/commandMcpRemove.go`
- [ ] `internal/runtime/tui/commandModel.go`
- [ ] `internal/runtime/tui/commandModelAdd.go`
- [ ] `internal/runtime/tui/commandModelList.go`
- [ ] `internal/runtime/tui/commandModelTag.go`
- [ ] `internal/runtime/tui/commandNew.go`
- [ ] `internal/runtime/tui/commandOutputDir.go`
- [ ] `internal/runtime/tui/commandPending.go`
- [ ] `internal/runtime/tui/commandProviderUsage.go`
- [ ] `internal/runtime/tui/commandReasoning.go`
- [ ] `internal/runtime/tui/commandRemoveSession.go`
- [ ] `internal/runtime/tui/commandReplyLanguage.go`
- [ ] `internal/runtime/tui/commandReset.go`
- [ ] `internal/runtime/tui/commandRole.go`
- [ ] `internal/runtime/tui/commandRouting.go`
- [ ] `internal/runtime/tui/commandSchedule.go`
- [ ] `internal/runtime/tui/commandScheduleAction.go`
- [ ] `internal/runtime/tui/commandScheduleMenu.go`
- [ ] `internal/runtime/tui/commandSkills.go`
- [ ] `internal/runtime/tui/commandSummaryModel.go`
- [ ] `internal/runtime/tui/commandSwitch.go`
- [ ] `internal/runtime/tui/commandTask.go`
- [ ] `internal/runtime/tui/commandTelegram.go`
- [ ] `internal/runtime/tui/commandUpdate.go`
- [ ] `internal/runtime/tui/commandUsage.go`
- [ ] `internal/runtime/tui/commandWorkDir.go`
- [ ] `internal/runtime/tui/commnadModelRemove.go`
- [ ] `internal/runtime/tui/dropPath.go`
- [ ] `internal/runtime/tui/handlerCommand.go`
- [ ] `internal/runtime/tui/handlerDaemonLog.go`
- [ ] `internal/runtime/tui/handlerExec.go`
- [ ] `internal/runtime/tui/handlerInputHistory.go`
- [ ] `internal/runtime/tui/handlerLogFormat.go`
- [ ] `internal/runtime/tui/handlerLogMode.go`
- [ ] `internal/runtime/tui/handlerPendingChannel.go`
- [ ] `internal/runtime/tui/handlerPopup.go`
- [ ] `internal/runtime/tui/handlerResize.go`
- [ ] `internal/runtime/tui/handlerShell.go`
- [ ] `internal/runtime/tui/helper.go`
- [ ] `internal/runtime/tui/init.go`
- [ ] `internal/runtime/tui/message.go`
- [ ] `internal/runtime/tui/renderTodo.go`
- [ ] `internal/runtime/tui/renderWait.go`
- [ ] `internal/runtime/tui/slog.go`
- [ ] `internal/runtime/tui/tui.go`
- [ ] `internal/runtime/tui/update.go`
- [ ] `internal/runtime/tui/view.go`
- [ ] `internal/runtime/tui/viewRender.go`

### `internal/runtime/webapp`

- [ ] `internal/runtime/webapp/asset.go`
- [ ] `internal/runtime/webapp/webapp.go`
- [ ] `internal/runtime/webapp/webapp_darwin.go`
- [ ] `internal/runtime/webapp/webapp_linux.go`

### `internal/session`

- [ ] `internal/session/clean.go`
- [ ] `internal/session/concurrent.go`
- [ ] `internal/session/reset.go`
- [ ] `internal/session/session.go`

### `internal/session/config/bot`

- [ ] `internal/session/config/bot/bot.go`

### `internal/session/config`

- [ ] `internal/session/config/config.go`
- [ ] `internal/session/config/model.go`
- [ ] `internal/session/config/tier.go`

### `internal/session/config/status`

- [ ] `internal/session/config/status/online.go`
- [ ] `internal/session/config/status/status.go`

### `internal/session/discord`

- [ ] `internal/session/discord/discord.go`

### `internal/session/history`

- [ ] `internal/session/history/compact.go`
- [ ] `internal/session/history/history.go`
- [ ] `internal/session/history/prune.go`
- [ ] `internal/session/history/record.go`
- [ ] `internal/session/history/replace.go`

### `internal/session/log`

- [ ] `internal/session/log/compact.go`
- [ ] `internal/session/log/format.go`
- [ ] `internal/session/log/record.go`
- [ ] `internal/session/log/replay.go`
- [ ] `internal/session/log/trimTool.go`

### `internal/session/summary`

- [ ] `internal/session/summary/prompt.go`
- [ ] `internal/session/summary/summary.go`
- [ ] `internal/session/summary/validate.go`

### `internal/session/telegram`

- [ ] `internal/session/telegram/telegram.go`

### `internal/session/tui`

- [ ] `internal/session/tui/tuiHash.go`

### `internal/session/usage`

- [ ] `internal/session/usage/new.go`
- [ ] `internal/session/usage/record.go`
- [ ] `internal/session/usage/summary.go`

### `internal/startup`

- [ ] `internal/startup/record.go`
- [ ] `internal/startup/startup_darwin.go`
- [ ] `internal/startup/startup_linux.go`

### `internal/tools`

- [x] `internal/tools/audio.go`
- [ ] `internal/tools/executor.go`
- [ ] `internal/tools/guide.go`
- [ ] `internal/tools/htmlTemplate.go`
- [x] `internal/tools/image.go`
- [ ] `internal/tools/legacyAPITool.go`
- [ ] `internal/tools/openFile.go`
- [x] `internal/tools/publicAPIList.go`
- [ ] `internal/tools/register.go`
- [ ] `internal/tools/tools.go`

### `internal/tools/calculate`

- [x] `internal/tools/calculate/calculate.go`
- [x] `internal/tools/calculate/register.go`

### `internal/tools/command`

- [ ] `internal/tools/command/register.go`
- [ ] `internal/tools/command/runCommand.go`
- [ ] `internal/tools/command/runCommandBackground.go`
- [ ] `internal/tools/command/runCommandShell.go`
- [ ] `internal/tools/command/runCommandWatch.go`
- [ ] `internal/tools/command/watchOutput.go`

### `internal/tools/cron`

- [ ] `internal/tools/cron/list.go`
- [ ] `internal/tools/cron/parseTime.go`
- [ ] `internal/tools/cron/patch.go`
- [ ] `internal/tools/cron/register.go`
- [ ] `internal/tools/cron/remove.go`
- [ ] `internal/tools/cron/schedules.go`
- [ ] `internal/tools/cron/validateCron.go`
- [ ] `internal/tools/cron/write.go`

### `internal/tools/fetchPage`

- [ ] `internal/tools/fetchPage/challenage.go`
- [ ] `internal/tools/fetchPage/decision.go`
- [x] `internal/tools/fetchPage/downloadFile.go`
- [x] `internal/tools/fetchPage/fetchPage.go`
- [ ] `internal/tools/fetchPage/register.go`
- [ ] `internal/tools/fetchPage/skipped.go`

### `internal/tools/file/boundary`

- [ ] `internal/tools/file/boundary/boundary.go`
- [ ] `internal/tools/file/boundary/workDir.go`

### `internal/tools/file`

- [ ] `internal/tools/file/findFiles.go`
- [ ] `internal/tools/file/fresh.go`
- [ ] `internal/tools/file/glob.go`
- [ ] `internal/tools/file/htmlCheck.go`
- [ ] `internal/tools/file/list.go`
- [x] `internal/tools/file/patch_contents.go`
- [ ] `internal/tools/file/patch_plan.go`
- [ ] `internal/tools/file/quote.go`
- [ ] `internal/tools/file/readFiles.go`
- [ ] `internal/tools/file/register.go`
- [ ] `internal/tools/file/relevance.go`
- [ ] `internal/tools/file/remove.go`
- [ ] `internal/tools/file/removeFile.go`
- [ ] `internal/tools/file/restoreFile.go`
- [ ] `internal/tools/file/search.go`
- [x] `internal/tools/file/write_file.go`
- [x] `internal/tools/file/write_result.go`
- [x] `internal/tools/file/write_to_path.go`

### `internal/tools/file/variant`

- [ ] `internal/tools/file/variant/editSkill.go`
- [ ] `internal/tools/file/variant/editTool.go`
- [ ] `internal/tools/file/variant/register.go`
- [ ] `internal/tools/file/variant/skillPatch.go`
- [ ] `internal/tools/file/variant/skillRemove.go`
- [ ] `internal/tools/file/variant/skillWrite.go`
- [ ] `internal/tools/file/variant/toolPatch.go`
- [ ] `internal/tools/file/variant/toolRemove.go`
- [ ] `internal/tools/file/variant/toolTest.go`
- [ ] `internal/tools/file/variant/toolWrite.go`

### `internal/tools/history/action`

- [ ] `internal/tools/history/action/chatHistory.go`
- [ ] `internal/tools/history/action/list.go`
- [ ] `internal/tools/history/action/read.go`
- [ ] `internal/tools/history/action/record.go`
- [ ] `internal/tools/history/action/reference.go`
- [ ] `internal/tools/history/action/register.go`
- [ ] `internal/tools/history/action/search.go`
- [ ] `internal/tools/history/action/tools.go`

### `internal/tools/history/error`

- [ ] `internal/tools/history/error/errorHistory.go`
- [ ] `internal/tools/history/error/writeError.go`

### `internal/tools/history/file`

- [ ] `internal/tools/history/file/fileHistory.go`
- [ ] `internal/tools/history/file/list.go`
- [ ] `internal/tools/history/file/read.go`
- [ ] `internal/tools/history/file/register.go`
- [ ] `internal/tools/history/file/restore.go`
- [ ] `internal/tools/history/file/target.go`

### `internal/tools/interactive`

- [ ] `internal/tools/interactive/askUser.go`
- [ ] `internal/tools/interactive/background.go`
- [ ] `internal/tools/interactive/backgroundCancel.go`
- [ ] `internal/tools/interactive/backgroundStart.go`
- [ ] `internal/tools/interactive/online.go`
- [ ] `internal/tools/interactive/register.go`
- [ ] `internal/tools/interactive/storeSecret.go`
- [ ] `internal/tools/interactive/writeTodo.go`

### `internal/tools/register`

- [ ] `internal/tools/register/register.go`

### `internal/tools/searchWeb`

- [ ] `internal/tools/searchWeb/decision.go`
- [ ] `internal/tools/searchWeb/fetch.go`
- [ ] `internal/tools/searchWeb/register.go`

### `internal/tools/searchWeb/googleRSS`

- [ ] `internal/tools/searchWeb/googleRSS/fetch.go`
- [ ] `internal/tools/searchWeb/googleRSS/register.go`

### `internal/tools/subagent`

- [ ] `internal/tools/subagent/background.go`
- [ ] `internal/tools/subagent/invoke.go`
- [ ] `internal/tools/subagent/list.go`
- [ ] `internal/tools/subagent/subagents.go`

### `internal/tools/toolAdapter`

- [ ] `internal/tools/toolAdapter/send.go`
- [ ] `internal/tools/toolAdapter/toolAdapter.go`

### `internal/tools/toolSkill`

- [ ] `internal/tools/toolSkill/findTools.go`
- [ ] `internal/tools/toolSkill/list.go`
- [ ] `internal/tools/toolSkill/register.go`
- [ ] `internal/tools/toolSkill/runSkill.go`
- [ ] `internal/tools/toolSkill/runTool.go`
- [ ] `internal/tools/toolSkill/search.go`

### `internal/tools/toolcache`

- [ ] `internal/tools/toolcache/cache.go`

### `internal/tools/types`

- [ ] `internal/tools/types/executor.go`
- [ ] `internal/tools/types/progress.go`

### `internal/utils`

- [ ] `internal/utils/askUserFormat.go`
- [ ] `internal/utils/auth.go`
- [ ] `internal/utils/chatbotEvent.go`
- [ ] `internal/utils/checkLink.go`
- [ ] `internal/utils/event.go`
- [ ] `internal/utils/json.go`
- [ ] `internal/utils/loopback.go`
- [ ] `internal/utils/permissionFormat.go`
- [ ] `internal/utils/toolFormat.go`
- [ ] `internal/utils/utils.go`
- [ ] `internal/utils/wsl.go`

### `internal/utils/msg`

- [ ] `internal/utils/msg/msg.go`

### `internal/utils/secret`

- [ ] `internal/utils/secret/secret.go`
