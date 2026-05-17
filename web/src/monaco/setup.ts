import { loader } from "@monaco-editor/react";
import * as monaco from "monaco-editor";

import EditorWorker from "./editor.worker?worker";

self.MonacoEnvironment = {
  getWorker() {
    return new EditorWorker();
  },
};

monaco.editor.defineTheme("zanbato", {
  base: "vs-dark",
  inherit: true,
  rules: [],
  colors: {
    "editor.background": "#0e1419",
  },
});

loader.config({ monaco });
