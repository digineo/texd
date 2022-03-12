const SAMPLE = `% sample document
\\documentclass{article}
\\usepackage{blindtext}

\\begin{document}
  \\tableofcontents
  \\Blinddocument
\\end{document}`

const genId = (id => () => ++id)(0),
      newFile = type => ({ id: genId(), type, name: "", content: "" })

document.addEventListener("DOMContentLoaded", () => {
  Vue.createApp({
    data() {
      const id = genId()

      return {
        rendering: false,
        engine:    "xelatex",
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
      }
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
          input:  this.files.find(f => f.id === this.main).name,
        }).reduce((params, [key, val]) => {
          return params.concat([`${key}=${encodeURIComponent(val)}`])
        }, [])

        const res = await fetch(`/render?${q.join("&")}`, {
          method: "POST",
          body:   fd,
        })

        const ct = res.headers.get("Content-Type").split(";", 2)[0],
              status = res.status

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
    },
  }).mount('#app')
})
