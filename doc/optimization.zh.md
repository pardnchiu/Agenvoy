# 優化進度

Agenvoy 是一人開發的專案，從 2 月到 10 月幾乎每天不間斷地開發，一路以推進功能為主，很少回頭整理已經寫好的程式碼。
累積到現在，整體的細節已經對個人記憶有點壓力，為了往後方便維護，也是時候放慢腳步開始整理。

並且這一年開始深入研究 Rust，一些過往的壞習慣與觀念需要調整，所以年初與現在的寫法已經有落差。
因此這段時間以檢查與整理為主：會逐檔看過一次，清掉 dead code 與優化實作，並把錯誤訊息、命名與寫法統一到現在的習慣。

同時也會開始盡量補上註解，讓程式碼從「只有我看得懂」轉向方便他人閱讀。

## 檔案（進度 1.1%）

### `cmd/app`

- [ ] `cmd/app/daemon.go` (252)
- [ ] `cmd/app/main.go` (144)
- [ ] `cmd/app/tui.go` (105)

### `configs`

- [ ] `configs/configs.go` (294)

### `extensions`

- [ ] `extensions/extensions.go` (6)

### `internal/agents/exec/allow/skill`

- [x] `internal/agents/exec/allow/skill/match.go` (59)
- [x] `internal/agents/exec/allow/skill/toggle.go` (59)

### `internal/agents/exec/allow/tool`

- [x] `internal/agents/exec/allow/tool/match.go` (85)
- [x] `internal/agents/exec/allow/tool/parse.go` (80)
- [x] `internal/agents/exec/allow/tool/toggle.go` (101)

### `internal/agents/exec`

- [ ] `internal/agents/exec/assign_skill.go` (39)
- [ ] `internal/agents/exec/assign_turn_context.go` (75)
- [ ] `internal/agents/exec/cancel.go` (40)
- [ ] `internal/agents/exec/execute.go` (870)
- [ ] `internal/agents/exec/push.go` (67)
- [ ] `internal/agents/exec/reset.go` (49)
- [ ] `internal/agents/exec/response.go` (61)
- [ ] `internal/agents/exec/run.go` (176)
- [ ] `internal/agents/exec/saveHistory.go` (90)
- [ ] `internal/agents/exec/schedule.go` (21)
- [ ] `internal/agents/exec/sendError.go` (27)
- [ ] `internal/agents/exec/sendEvent.go` (71)
- [ ] `internal/agents/exec/session.go` (158)
- [ ] `internal/agents/exec/steerWait.go` (91)
- [ ] `internal/agents/exec/stream.go` (21)
- [ ] `internal/agents/exec/streamText.go` (412)
- [ ] `internal/agents/exec/systemInfo.go` (127)
- [ ] `internal/agents/exec/systemPrompt.go` (314)
- [ ] `internal/agents/exec/toolCall.go` (987)

### `internal/agents/exec/chatbot`

- [ ] `internal/agents/exec/chatbot/admin.go` (88)

### `internal/agents/exec/compact`

- [ ] `internal/agents/exec/compact/assembleMessages.go` (31)
- [ ] `internal/agents/exec/compact/checkThreshold.go` (28)
- [ ] `internal/agents/exec/compact/copilotLimit.go` (118)
- [ ] `internal/agents/exec/compact/extractContextHistory.go` (90)
- [ ] `internal/agents/exec/compact/modelLimit.go` (166)
- [ ] `internal/agents/exec/compact/rawToolFallback.go` (72)
- [ ] `internal/agents/exec/compact/sessionHistory.go` (198)
- [ ] `internal/agents/exec/compact/toolHistory.go` (121)
- [ ] `internal/agents/exec/compact/trimFallback.go` (69)

### `internal/agents/exec/fast`

- [ ] `internal/agents/exec/fast/fast.go` (41)

### `internal/agents/exec/followup`

- [ ] `internal/agents/exec/followup/followup.go` (147)

### `internal/agents/exec/guide`

- [ ] `internal/agents/exec/guide/guide.go` (49)

### `internal/agents/exec/memory`

- [ ] `internal/agents/exec/memory/save.go` (151)
- [ ] `internal/agents/exec/memory/search.go` (191)

### `internal/agents/exec/model`

- [ ] `internal/agents/exec/model/check_alive.go` (60)
- [ ] `internal/agents/exec/model/list.go` (31)
- [ ] `internal/agents/exec/model/next.go` (81)
- [ ] `internal/agents/exec/model/route.go` (76)
- [ ] `internal/agents/exec/model/select.go` (147)
- [ ] `internal/agents/exec/model/select_with_jev.go` (265)
- [ ] `internal/agents/exec/model/select_with_llm.go` (120)

### `internal/agents/exec/retryHandler`

- [ ] `internal/agents/exec/retryHandler/reason.go` (70)
- [ ] `internal/agents/exec/retryHandler/retryHandler.go` (104)

### `internal/agents/exec/subagent`

- [ ] `internal/agents/exec/subagent/ensureSession.go` (30)
- [ ] `internal/agents/exec/subagent/exec.go` (288)
- [ ] `internal/agents/exec/subagent/sendEvent.go` (32)
- [ ] `internal/agents/exec/subagent/waitResume.go` (54)

### `internal/agents/exec/summary`

- [ ] `internal/agents/exec/summary/generate.go` (185)
- [ ] `internal/agents/exec/summary/send.go` (89)

### `internal/agents/exec/todo`

- [ ] `internal/agents/exec/todo/writeTodo.go` (78)

### `internal/agents`

- [ ] `internal/agents/host.go` (119)

### `internal/agents/keychain`

- [ ] `internal/agents/keychain/keychain.go` (183)

### `internal/agents/probe`

- [ ] `internal/agents/probe/probe.go` (175)

### `internal/agents/types`

- [ ] `internal/agents/types/agent.go` (100)
- [ ] `internal/agents/types/event.go` (227)
- [ ] `internal/agents/types/usage.go` (29)

### `internal/app`

- [ ] `internal/app/agentRegistry.go` (139)
- [ ] `internal/app/daemonLog.go` (103)
- [ ] `internal/app/deliverBackground.go` (131)
- [ ] `internal/app/generateSummary.go` (34)
- [ ] `internal/app/loopbackListeners.go` (30)
- [ ] `internal/app/newMCP.go` (21)
- [ ] `internal/app/pushHook.go` (22)
- [ ] `internal/app/reloadDiscord.go` (71)
- [ ] `internal/app/reloadRetry.go` (6)
- [ ] `internal/app/reloadTelegram.go` (71)
- [ ] `internal/app/runSkill.go` (38)
- [ ] `internal/app/spawnDaemon.go` (51)
- [ ] `internal/app/watchConfig.go` (67)
- [ ] `internal/app/watchSession.go` (77)

### `internal/filesystem`

- [ ] `internal/filesystem/db.go` (57)
- [ ] `internal/filesystem/media.go` (68)
- [ ] `internal/filesystem/path.go` (311)
- [ ] `internal/filesystem/reader.go` (223)
- [ ] `internal/filesystem/replyLang.go` (141)
- [ ] `internal/filesystem/runtime.go` (268)

### `internal/filesystem/record`

- [ ] `internal/filesystem/record/download.go` (82)
- [ ] `internal/filesystem/record/log.go` (42)
- [ ] `internal/filesystem/record/storeTemp.go` (92)

### `internal/filesystem/skill`

- [ ] `internal/filesystem/skill/scheduleSkill.go` (87)
- [ ] `internal/filesystem/skill/skill.go` (119)

### `internal/runtime`

- [ ] `internal/runtime/cancelNotify.go` (29)
- [ ] `internal/runtime/cron.go` (108)
- [ ] `internal/runtime/listener.go` (422)
- [ ] `internal/runtime/pending.go` (293)
- [ ] `internal/runtime/resume.go` (47)
- [ ] `internal/runtime/runtime.go` (119)
- [ ] `internal/runtime/scheduler.go` (347)
- [ ] `internal/runtime/skill.go` (205)
- [ ] `internal/runtime/task.go` (143)
- [ ] `internal/runtime/version.go` (7)

### `internal/runtime/chatbot`

- [ ] `internal/runtime/chatbot/chatbot.go` (133)
- [ ] `internal/runtime/chatbot/chunk.go` (160)
- [ ] `internal/runtime/chatbot/reply.go` (127)
- [ ] `internal/runtime/chatbot/sanitize.go` (61)

### `internal/runtime/chatbot/discord`

- [ ] `internal/runtime/chatbot/discord/attachments.go` (95)
- [ ] `internal/runtime/chatbot/discord/new.go` (109)
- [ ] `internal/runtime/chatbot/discord/pending.go` (85)
- [ ] `internal/runtime/chatbot/discord/push.go` (73)
- [ ] `internal/runtime/chatbot/discord/reply.go` (46)
- [ ] `internal/runtime/chatbot/discord/resume.go` (52)
- [ ] `internal/runtime/chatbot/discord/run.go` (205)

### `internal/runtime/chatbot/telegram`

- [ ] `internal/runtime/chatbot/telegram/attachments.go` (158)
- [ ] `internal/runtime/chatbot/telegram/fileMarker.go` (22)
- [ ] `internal/runtime/chatbot/telegram/filegroup.go` (79)
- [ ] `internal/runtime/chatbot/telegram/new.go` (119)
- [ ] `internal/runtime/chatbot/telegram/pending.go` (97)
- [ ] `internal/runtime/chatbot/telegram/push.go` (89)
- [ ] `internal/runtime/chatbot/telegram/reply.go` (69)
- [ ] `internal/runtime/chatbot/telegram/resume.go` (70)
- [ ] `internal/runtime/chatbot/telegram/run.go` (235)

### `internal/runtime/chatbot/tool`

- [ ] `internal/runtime/chatbot/tool/list.go` (60)
- [ ] `internal/runtime/chatbot/tool/platform.go` (30)
- [ ] `internal/runtime/chatbot/tool/record.go` (50)
- [ ] `internal/runtime/chatbot/tool/register.go` (33)
- [ ] `internal/runtime/chatbot/tool/send.go` (163)

### `internal/runtime/daemon`

- [ ] `internal/runtime/daemon/client.go` (48)

### `internal/runtime/ipc`

- [ ] `internal/runtime/ipc/askUser.go` (47)
- [ ] `internal/runtime/ipc/client.go` (187)
- [ ] `internal/runtime/ipc/frame.go` (77)
- [ ] `internal/runtime/ipc/mcp.go` (40)
- [ ] `internal/runtime/ipc/pending.go` (73)
- [ ] `internal/runtime/ipc/run.go` (114)
- [ ] `internal/runtime/ipc/server.go` (214)

### `internal/runtime/mcp`

- [ ] `internal/runtime/mcp/client.go` (136)
- [ ] `internal/runtime/mcp/config.go` (206)
- [ ] `internal/runtime/mcp/expose.go` (194)
- [ ] `internal/runtime/mcp/mcp.go` (384)
- [ ] `internal/runtime/mcp/oauth.go` (693)
- [ ] `internal/runtime/mcp/poll.go` (120)
- [ ] `internal/runtime/mcp/register.go` (61)
- [ ] `internal/runtime/mcp/server.go` (206)
- [ ] `internal/runtime/mcp/watch.go` (111)

### `internal/runtime/pubsub`

- [ ] `internal/runtime/pubsub/pubsub.go` (78)
- [ ] `internal/runtime/pubsub/subscriber.go` (79)

### `internal/runtime/routes/handler`

- [ ] `internal/runtime/routes/handler/allowlist.go` (160)
- [ ] `internal/runtime/routes/handler/automation.go` (372)
- [ ] `internal/runtime/routes/handler/cancel.go` (40)
- [ ] `internal/runtime/routes/handler/channel.go` (281)
- [ ] `internal/runtime/routes/handler/config.go` (257)
- [ ] `internal/runtime/routes/handler/confirm.go` (173)
- [ ] `internal/runtime/routes/handler/daemonLog.go` (96)
- [ ] `internal/runtime/routes/handler/drainEvents.go` (12)
- [ ] `internal/runtime/routes/handler/file.go` (149)
- [ ] `internal/runtime/routes/handler/fileLocate.go` (124)
- [ ] `internal/runtime/routes/handler/info.go` (18)
- [ ] `internal/runtime/routes/handler/key.go` (32)
- [ ] `internal/runtime/routes/handler/keys.go` (53)
- [ ] `internal/runtime/routes/handler/mcpOAuth.go` (182)
- [ ] `internal/runtime/routes/handler/mcpServers.go` (121)
- [ ] `internal/runtime/routes/handler/memory.go` (105)
- [ ] `internal/runtime/routes/handler/model.go` (474)
- [ ] `internal/runtime/routes/handler/multilog.go` (230)
- [ ] `internal/runtime/routes/handler/pending.go` (230)
- [ ] `internal/runtime/routes/handler/providerQuota.go` (129)
- [ ] `internal/runtime/routes/handler/providers.go` (365)
- [ ] `internal/runtime/routes/handler/role.go` (199)
- [ ] `internal/runtime/routes/handler/send.go` (128)
- [ ] `internal/runtime/routes/handler/sendFollowup.go` (42)
- [ ] `internal/runtime/routes/handler/sendResult.go` (63)
- [ ] `internal/runtime/routes/handler/sendSSE.go` (60)
- [ ] `internal/runtime/routes/handler/session.go` (317)
- [ ] `internal/runtime/routes/handler/sessionArtifact.go` (132)
- [ ] `internal/runtime/routes/handler/skill.go` (179)
- [ ] `internal/runtime/routes/handler/systemUpdate.go` (242)
- [ ] `internal/runtime/routes/handler/tools.go` (46)
- [ ] `internal/runtime/routes/handler/toriiInspect.go` (82)
- [ ] `internal/runtime/routes/handler/voice.go` (133)
- [ ] `internal/runtime/routes/handler/workDir.go` (18)

### `internal/runtime/routes/handler/chatCompletions`

- [ ] `internal/runtime/routes/handler/chatCompletions/chatCompletions.go` (272)
- [ ] `internal/runtime/routes/handler/chatCompletions/run.go` (100)
- [ ] `internal/runtime/routes/handler/chatCompletions/stream.go` (185)

### `internal/runtime/routes`

- [ ] `internal/runtime/routes/new.go` (191)
- [ ] `internal/runtime/routes/page.go` (78)
- [ ] `internal/runtime/routes/sw.go` (166)

### `internal/runtime/store`

- [ ] `internal/runtime/store/actionHistory.go` (164)
- [ ] `internal/runtime/store/fileOrigin.go` (51)
- [ ] `internal/runtime/store/fileQuery.go` (221)
- [ ] `internal/runtime/store/fileRecord.go` (142)
- [ ] `internal/runtime/store/fileRestore.go` (175)
- [ ] `internal/runtime/store/fileStore.go` (89)
- [ ] `internal/runtime/store/message.go` (116)
- [ ] `internal/runtime/store/messageSearch.go` (74)
- [ ] `internal/runtime/store/session.go` (206)
- [ ] `internal/runtime/store/store.go` (117)

### `internal/runtime/toolAdapter/api`

- [ ] `internal/runtime/toolAdapter/api/adapter.go` (190)
- [ ] `internal/runtime/toolAdapter/api/execute.go` (198)
- [ ] `internal/runtime/toolAdapter/api/translate.go` (77)

### `internal/runtime/toolAdapter/script`

- [ ] `internal/runtime/toolAdapter/script/ececute.go` (87)
- [ ] `internal/runtime/toolAdapter/script/translator.go` (169)

### `internal/runtime/torii`

- [ ] `internal/runtime/torii/list.go` (39)
- [ ] `internal/runtime/torii/session.go` (184)
- [ ] `internal/runtime/torii/torii.go` (83)

### `internal/runtime/tui`

- [ ] `internal/runtime/tui/cmdSelector.go` (348)
- [ ] `internal/runtime/tui/commandAdminChannel.go` (86)
- [ ] `internal/runtime/tui/commandAudioModel.go` (38)
- [ ] `internal/runtime/tui/commandBot.go` (142)
- [ ] `internal/runtime/tui/commandChannel.go` (209)
- [ ] `internal/runtime/tui/commandCompact.go` (203)
- [ ] `internal/runtime/tui/commandConfig.go` (135)
- [ ] `internal/runtime/tui/commandCron.go` (50)
- [ ] `internal/runtime/tui/commandDiscord.go` (118)
- [ ] `internal/runtime/tui/commandDispatcher.go` (114)
- [ ] `internal/runtime/tui/commandImageModel.go` (33)
- [ ] `internal/runtime/tui/commandKey.go` (114)
- [ ] `internal/runtime/tui/commandLog.go` (35)
- [ ] `internal/runtime/tui/commandMcp.go` (178)
- [ ] `internal/runtime/tui/commandMcpAdd.go` (391)
- [ ] `internal/runtime/tui/commandMcpLogin.go` (185)
- [ ] `internal/runtime/tui/commandMcpPermission.go` (158)
- [ ] `internal/runtime/tui/commandMcpRemove.go` (58)
- [ ] `internal/runtime/tui/commandModel.go` (188)
- [ ] `internal/runtime/tui/commandModelAdd.go` (1001)
- [ ] `internal/runtime/tui/commandModelList.go` (123)
- [ ] `internal/runtime/tui/commandModelTag.go` (54)
- [ ] `internal/runtime/tui/commandNew.go` (142)
- [ ] `internal/runtime/tui/commandOutputDir.go` (49)
- [ ] `internal/runtime/tui/commandPending.go` (147)
- [ ] `internal/runtime/tui/commandProviderUsage.go` (145)
- [ ] `internal/runtime/tui/commandReasoning.go` (77)
- [ ] `internal/runtime/tui/commandRemoveSession.go` (135)
- [ ] `internal/runtime/tui/commandReplyLanguage.go` (68)
- [ ] `internal/runtime/tui/commandReset.go` (115)
- [ ] `internal/runtime/tui/commandRole.go` (191)
- [ ] `internal/runtime/tui/commandRouting.go` (131)
- [ ] `internal/runtime/tui/commandSchedule.go` (43)
- [ ] `internal/runtime/tui/commandScheduleAction.go` (93)
- [ ] `internal/runtime/tui/commandScheduleMenu.go` (66)
- [ ] `internal/runtime/tui/commandSkills.go` (240)
- [ ] `internal/runtime/tui/commandSummaryModel.go` (31)
- [ ] `internal/runtime/tui/commandSwitch.go` (272)
- [ ] `internal/runtime/tui/commandTask.go` (21)
- [ ] `internal/runtime/tui/commandTelegram.go` (118)
- [ ] `internal/runtime/tui/commandUpdate.go` (42)
- [ ] `internal/runtime/tui/commandUsage.go` (179)
- [ ] `internal/runtime/tui/commandWorkDir.go` (64)
- [ ] `internal/runtime/tui/commnadModelRemove.go` (88)
- [ ] `internal/runtime/tui/dropPath.go` (60)
- [ ] `internal/runtime/tui/handlerCommand.go` (108)
- [ ] `internal/runtime/tui/handlerDaemonLog.go` (83)
- [ ] `internal/runtime/tui/handlerExec.go` (553)
- [ ] `internal/runtime/tui/handlerInputHistory.go` (124)
- [ ] `internal/runtime/tui/handlerLogFormat.go` (212)
- [ ] `internal/runtime/tui/handlerLogMode.go` (206)
- [ ] `internal/runtime/tui/handlerPendingChannel.go` (19)
- [ ] `internal/runtime/tui/handlerPopup.go` (803)
- [ ] `internal/runtime/tui/handlerShell.go` (86)
- [ ] `internal/runtime/tui/helper.go` (65)
- [ ] `internal/runtime/tui/init.go` (300)
- [ ] `internal/runtime/tui/message.go` (53)
- [ ] `internal/runtime/tui/renderTodo.go` (48)
- [ ] `internal/runtime/tui/renderWait.go` (21)
- [ ] `internal/runtime/tui/slog.go` (122)
- [ ] `internal/runtime/tui/tui.go` (92)
- [ ] `internal/runtime/tui/typesafe.go` (73)
- [ ] `internal/runtime/tui/update.go` (1154)
- [ ] `internal/runtime/tui/view.go` (479)
- [ ] `internal/runtime/tui/viewRender.go` (728)

### `internal/runtime/webapp`

- [ ] `internal/runtime/webapp/asset.go` (77)
- [ ] `internal/runtime/webapp/webapp.go` (23)
- [ ] `internal/runtime/webapp/webapp_darwin.go` (95)
- [ ] `internal/runtime/webapp/webapp_linux.go` (185)

### `internal/session`

- [ ] `internal/session/clean.go` (90)
- [ ] `internal/session/concurrent.go` (80)
- [ ] `internal/session/reset.go` (50)
- [ ] `internal/session/session.go` (90)

### `internal/session/config/bot`

- [ ] `internal/session/config/bot/bot.go` (219)

### `internal/session/config`

- [ ] `internal/session/config/config.go` (144)
- [ ] `internal/session/config/model.go` (135)
- [ ] `internal/session/config/tier.go` (263)

### `internal/session/config/status`

- [ ] `internal/session/config/status/online.go` (11)
- [ ] `internal/session/config/status/status.go` (19)

### `internal/session/discord`

- [ ] `internal/session/discord/discord.go` (56)

### `internal/session/history`

- [ ] `internal/session/history/compact.go` (119)
- [ ] `internal/session/history/history.go` (83)
- [ ] `internal/session/history/prune.go` (59)
- [ ] `internal/session/history/record.go` (107)
- [ ] `internal/session/history/replace.go` (42)

### `internal/session/log`

- [ ] `internal/session/log/compact.go` (80)
- [ ] `internal/session/log/format.go` (146)
- [ ] `internal/session/log/record.go` (167)
- [ ] `internal/session/log/replay.go` (171)
- [ ] `internal/session/log/trimTool.go` (59)

### `internal/session/summary`

- [ ] `internal/session/summary/prompt.go` (57)
- [ ] `internal/session/summary/summary.go` (107)
- [ ] `internal/session/summary/validate.go` (16)

### `internal/session/telegram`

- [ ] `internal/session/telegram/telegram.go` (43)

### `internal/session/tui`

- [ ] `internal/session/tui/tuiHash.go` (38)

### `internal/session/usage`

- [ ] `internal/session/usage/new.go` (63)
- [ ] `internal/session/usage/record.go` (42)
- [ ] `internal/session/usage/summary.go` (92)

### `internal/startup`

- [ ] `internal/startup/record.go` (38)
- [ ] `internal/startup/startup_darwin.go` (106)
- [ ] `internal/startup/startup_linux.go` (117)

### `internal/sudo`

- [ ] `internal/sudo/password.go` (54)

### `internal/tools/calculate`

- [ ] `internal/tools/calculate/calculate.go` (259)
- [ ] `internal/tools/calculate/register.go` (61)

### `internal/tools`

- [ ] `internal/tools/downloadFile.go` (134)
- [ ] `internal/tools/executor.go` (164)
- [ ] `internal/tools/guide.go` (103)
- [ ] `internal/tools/htmlTemplate.go` (100)
- [ ] `internal/tools/openFile.go` (130)
- [ ] `internal/tools/register.go` (72)
- [ ] `internal/tools/runCommand.go` (321)
- [ ] `internal/tools/runCommandBackground.go` (85)
- [ ] `internal/tools/runCommandShell.go` (114)
- [ ] `internal/tools/runCommandWatch.go` (158)
- [ ] `internal/tools/tools.go` (31)

### `internal/tools/external/audio`

- [ ] `internal/tools/external/audio/audio.go` (176)
- [ ] `internal/tools/external/audio/generate.go` (77)
- [ ] `internal/tools/external/audio/register.go` (62)

### `internal/tools/external/image`

- [ ] `internal/tools/external/image/generate.go` (130)
- [ ] `internal/tools/external/image/provider.go` (159)
- [ ] `internal/tools/external/image/register.go` (89)

### `internal/tools/external/searchWeb`

- [ ] `internal/tools/external/searchWeb/fetch.go` (309)
- [ ] `internal/tools/external/searchWeb/register.go` (169)

### `internal/tools/external/searchWeb/googleRSS`

- [ ] `internal/tools/external/searchWeb/googleRSS/fetch.go` (124)
- [ ] `internal/tools/external/searchWeb/googleRSS/register.go` (35)

### `internal/tools/external`

- [ ] `internal/tools/external/send.go` (257)
- [ ] `internal/tools/external/toolAdapter.go` (88)

### `internal/tools/fetchPage`

- [ ] `internal/tools/fetchPage/check.go` (44)
- [ ] `internal/tools/fetchPage/fetchPage.go` (353)
- [ ] `internal/tools/fetchPage/register.go` (5)
- [ ] `internal/tools/fetchPage/skipped.go` (63)

### `internal/tools/file/boundary`

- [ ] `internal/tools/file/boundary/boundary.go` (246)
- [ ] `internal/tools/file/boundary/sensitive.go` (43)

### `internal/tools/file`

- [ ] `internal/tools/file/editFile.go` (124)
- [ ] `internal/tools/file/findFiles.go` (212)
- [ ] `internal/tools/file/fresh.go` (27)
- [ ] `internal/tools/file/glob.go` (88)
- [ ] `internal/tools/file/htmlCheck.go` (98)
- [ ] `internal/tools/file/list.go` (73)
- [ ] `internal/tools/file/patchFile.go` (239)
- [ ] `internal/tools/file/quote.go` (95)
- [ ] `internal/tools/file/readFiles.go` (152)
- [ ] `internal/tools/file/register.go` (11)
- [ ] `internal/tools/file/remove.go` (63)
- [ ] `internal/tools/file/search.go` (279)
- [ ] `internal/tools/file/writeFile.go` (104)
- [ ] `internal/tools/file/writeResult.go` (81)

### `internal/tools/file/variant`

- [ ] `internal/tools/file/variant/editSkill.go` (96)
- [ ] `internal/tools/file/variant/editTool.go` (96)
- [ ] `internal/tools/file/variant/register.go` (89)
- [ ] `internal/tools/file/variant/skillPatch.go` (31)
- [ ] `internal/tools/file/variant/skillRemove.go` (34)
- [ ] `internal/tools/file/variant/skillWrite.go` (45)
- [ ] `internal/tools/file/variant/toolPatch.go` (31)
- [ ] `internal/tools/file/variant/toolRemove.go` (34)
- [ ] `internal/tools/file/variant/toolTest.go` (96)
- [ ] `internal/tools/file/variant/toolWrite.go` (60)

### `internal/tools/history/action`

- [ ] `internal/tools/history/action/chatHistory.go` (169)
- [ ] `internal/tools/history/action/list.go` (56)
- [ ] `internal/tools/history/action/read.go` (82)
- [ ] `internal/tools/history/action/record.go` (91)
- [ ] `internal/tools/history/action/reference.go` (22)
- [ ] `internal/tools/history/action/register.go` (5)
- [ ] `internal/tools/history/action/search.go` (291)
- [ ] `internal/tools/history/action/tools.go` (126)

### `internal/tools/history/error`

- [ ] `internal/tools/history/error/errorHistory.go` (178)

### `internal/tools/history/file`

- [ ] `internal/tools/history/file/fileHistory.go` (113)
- [ ] `internal/tools/history/file/list.go` (72)
- [ ] `internal/tools/history/file/read.go` (97)
- [ ] `internal/tools/history/file/register.go` (5)
- [ ] `internal/tools/history/file/restore.go` (66)
- [ ] `internal/tools/history/file/target.go` (53)

### `internal/tools/interactive`

- [ ] `internal/tools/interactive/askUser.go` (846)
- [ ] `internal/tools/interactive/background.go` (230)
- [ ] `internal/tools/interactive/backgroundCancel.go` (96)
- [ ] `internal/tools/interactive/backgroundStart.go` (60)
- [ ] `internal/tools/interactive/online.go` (117)
- [ ] `internal/tools/interactive/pkgManage.go` (313)
- [ ] `internal/tools/interactive/register.go` (8)
- [ ] `internal/tools/interactive/storeSecret.go` (117)
- [ ] `internal/tools/interactive/writeTodo.go` (160)

### `internal/tools/register`

- [ ] `internal/tools/register/register.go` (295)

### `internal/tools/scheduler`

- [ ] `internal/tools/scheduler/list.go` (57)
- [ ] `internal/tools/scheduler/parseTime.go` (39)
- [ ] `internal/tools/scheduler/patch.go` (48)
- [ ] `internal/tools/scheduler/register.go` (5)
- [ ] `internal/tools/scheduler/remove.go` (36)
- [ ] `internal/tools/scheduler/schedules.go` (101)
- [ ] `internal/tools/scheduler/validateCron.go` (40)
- [ ] `internal/tools/scheduler/write.go` (57)

### `internal/tools/searcher`

- [ ] `internal/tools/searcher/findTools.go` (106)
- [ ] `internal/tools/searcher/list.go` (45)
- [ ] `internal/tools/searcher/register.go` (7)
- [ ] `internal/tools/searcher/runSkill.go` (87)
- [ ] `internal/tools/searcher/runTool.go` (38)
- [ ] `internal/tools/searcher/search.go` (211)

### `internal/tools/subagent`

- [ ] `internal/tools/subagent/background.go` (36)
- [ ] `internal/tools/subagent/invoke.go` (158)
- [ ] `internal/tools/subagent/list.go` (34)
- [ ] `internal/tools/subagent/subagents.go` (124)

### `internal/tools/toolcache`

- [ ] `internal/tools/toolcache/cache.go` (145)

### `internal/tools/types`

- [ ] `internal/tools/types/elided.go` (9)
- [ ] `internal/tools/types/executor.go` (96)
- [ ] `internal/tools/types/progress.go` (20)

### `internal/utils`

- [ ] `internal/utils/auth.go` (154)
- [ ] `internal/utils/chatbotEvent.go` (88)
- [ ] `internal/utils/checkLink.go` (48)
- [ ] `internal/utils/event.go` (403)
- [ ] `internal/utils/json.go` (16)
- [ ] `internal/utils/loopback.go` (15)
- [ ] `internal/utils/utils.go` (429)
- [ ] `internal/utils/wsl.go` (65)

### `internal/utils/msg`

- [ ] `internal/utils/msg/msg.go` (36)

### `page`

- [ ] `page/page.go` (6)

### `page/src/js`

- [ ] `page/src/js/_actionLog.js` (367)
- [ ] `page/src/js/_channel.js` (56)
- [ ] `page/src/js/_channel_config.js` (243)
- [ ] `page/src/js/_chat.js` (432)
- [ ] `page/src/js/_confirm.js` (153)
- [ ] `page/src/js/_daemon.js` (190)
- [ ] `page/src/js/_event.js` (230)
- [ ] `page/src/js/_hint.js` (39)
- [ ] `page/src/js/_history.js` (343)
- [ ] `page/src/js/_keychain.js` (160)
- [ ] `page/src/js/_leftTab.js` (74)
- [ ] `page/src/js/_lesson.js` (254)
- [ ] `page/src/js/_mcp.js` (569)
- [ ] `page/src/js/_memory.js` (110)
- [ ] `page/src/js/_model.js` (261)
- [ ] `page/src/js/_model_config.js` (1225)
- [ ] `page/src/js/_pending.js` (401)
- [ ] `page/src/js/_persona.js` (119)
- [ ] `page/src/js/_role.js` (98)
- [ ] `page/src/js/_roleEditor.js` (201)
- [ ] `page/src/js/_schedule.js` (492)
- [ ] `page/src/js/_send.js` (567)
- [ ] `page/src/js/_skill.js` (362)
- [ ] `page/src/js/_subagent.js` (82)
- [ ] `page/src/js/_subscribe.js` (274)
- [ ] `page/src/js/_system.js` (314)
- [ ] `page/src/js/_toast.js` (39)
- [ ] `page/src/js/_todo.js` (25)
- [ ] `page/src/js/_usage.js` (316)
- [ ] `page/src/js/_utils.js` (640)
- [ ] `page/src/js/_workDir.js` (119)
- [ ] `page/src/js/index.js` (502)
