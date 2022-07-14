const SAMPLE = `% sample document
\\documentclass{article}
\\usepackage{blindtext}

\\begin{document}
  \\tableofcontents
  \\Blinddocument
\\end{document}`

const genId = (id => () => ++id)(0),
      newFile = type => {
        const id = genId()
        return { id, type, name: `file-${id}.tex`, content: "" }
      }

function inspectResponse(res) {
  const { ok, status, headers: h } = res,
        ct = (h.get("Content-Type") || "").split(";", 2)[0]
  return { ok, status, ct }
}

const app = Vue.createApp({
  data() {
    const id = genId()

    return {
      init:      false,
      version:   "0.0.0",
      rendering: false,
      queue:     [0, 100],
      engines:   ["xelatex", "lualatex", "pdflatex"],
      engine:    "xelatex",
      images:    [],
      image:     null,
      errors:    "full",
      result:    null,

      main:  id,
      files: [{
        id,
        name:    "input.tex",
        type:    "text",
        content: SAMPLE,
      }]
    }
  },

  computed: {
    fileNames() {
      const n = {}
      for (let i = 0, len = this.files.length; i < len; ++i) {
        const { name } = this.files[i]
        if (!name) {
          continue
        }
        n[name] || (n[name] = 0);
        ++n[name]
      }
      return n
    },

    mainInput() {
      return this.files.find(f => f.id === this.main)
    },
  },

  beforeMount() {
    setInterval(this.fetchStatus, 5000)
    this.fetchStatus()
  },

  methods: {
    newTextfile()  { this.files.push(newFile("text")) },
    newFileinput() { this.files.push(newFile("file")) },

    removeFile({ id }) {
      const idx = this.files.findIndex(f => f.id === id)
      if (idx >= 0) {
        this.files.splice(idx, 1)
      }
    },

    onFileChange(f, ev) {
      const [realFile] = ev.target.files
      f.content = realFile
      f.name = realFile.name
    },

    async fetchStatus() {
      const res = await fetch("/status", {
        headers: { Accept: "application/json" }
      })
      const { ok, status, ct } = inspectResponse(res)
      if (!ok) {
        this.result = {
          type:    "error",
          message: "Unable to fetch status of texd server.",
          status,
          ct,
        }
        return
      }

      const data = await res.json()
      this.version = data.version
      this.images = data.images || []
      this.engines = data.engines
      if (!this.init) {
        this.image = this.images[0]
        this.engine = data.default_engine
      }
      this.queue = [data.queue.length, data.queue.capacity]
      this.init = true
    },

    async onSubmit() {
      this.rendering = true
      this.result = null

      const fd = new FormData()
      for (let i = 0, len = this.files.length; i < len; ++i) {
        const { type, name, content } = this.files[i]
        if (type === "file") {
          fd.append(name, content)
        } else if (type === "text") {
          fd.append(name, new Blob([content]), name)
        }
      }

      q = Object.entries({
        errors: this.errors,
        engine: this.engine,
        image:  this.image,
        input:  this.mainInput.name,
      }).reduce((params, [key, val]) => {
          return val ? params.concat([`${key}=${encodeURIComponent(val)}`]) : params
      }, [])

      const res = await fetch(`/render?${q.join("&")}`, {
        method: "POST",
        body:   fd,
      })

      const { status, ct } = inspectResponse(res)
      switch (status) {
      case 200:
        if (ct === "application/pdf") {
          const blob = await res.blob()
          const data = await new Promise((resolve, reject) => {
            const r = new FileReader()
            r.readAsDataURL(blob)
            r.addEventListener("load", () => resolve(r.result))
          })
          this.result = { type: "pdf", data }
        } else {
          this.result = {
            type: "error",
            messge: `unexpected response: ${res.statusText}`,
            status,
            ct,
          }
        }
        break
      case 422:
        if (ct === "text/plain") {
          const data = await res.text()
          this.result = { type: "log", data }
        } else if (ct === "application/json") {
          const data = await res.json()
          this.result = { type: "status", data }
        }
        break
      default:
        this.result = {
          type: "error",
          message: `unexpected response`,
          status,
          ct,
        }
      }

      this.rendering = false
    },

    onReset() {
      this.files.splice(0)
      this.main = null
      this.result = null
    }
  },
})

document.addEventListener("DOMContentLoaded", () => {
  app.mount('#app')
})
