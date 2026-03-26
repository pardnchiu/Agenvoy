() => {
  Object.defineProperty(navigator, "webdriver", { get: () => undefined });

  const pluginData = [
    { name: "Chrome PDF Plugin", filename: "internal-pdf-viewer", description: "Portable Document Format" },
    { name: "Chrome PDF Viewer", filename: "mhjfbmdgcfjbbpaeojofohoefgiehjai", description: "" },
    { name: "Native Client", filename: "internal-nacl-plugin", description: "" },
  ];
  const pluginArray = Object.create(PluginArray.prototype);
  pluginData.forEach((p, i) => {
    const plugin = Object.create(Plugin.prototype);
    Object.defineProperty(plugin, "name", { get: () => p.name });
    Object.defineProperty(plugin, "filename", { get: () => p.filename });
    Object.defineProperty(plugin, "description", { get: () => p.description });
    Object.defineProperty(plugin, "length", { get: () => 0 });
    pluginArray[i] = plugin;
  });
  Object.defineProperty(pluginArray, "length", { get: () => pluginData.length });
  Object.defineProperty(navigator, "plugins", { get: () => pluginArray });

  Object.defineProperty(navigator, "languages", { get: () => ["en-US", "en"] });

  window.chrome = { runtime: {} };
};
