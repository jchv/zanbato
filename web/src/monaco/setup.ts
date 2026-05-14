import { loader } from "@monaco-editor/react";
import * as monaco from "monaco-editor";

import EditorWorker from "./editor.worker?worker";

self.MonacoEnvironment = {
  getWorker() {
    return new EditorWorker();
  },
};

loader.config({ monaco });
