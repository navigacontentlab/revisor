'use strict';

const WASM_URL = 'wasm.wasm';

var wasm;

async function init() {
    const go = new Go();
    const dec = new TextDecoder()
    const enc = new TextEncoder();

    window.revisor = {
        loadConstraints: async function() {
            for (let i = 0; i < arguments.length; i++) {
                if (typeof arguments[i] == "string") {
                    arguments[i] = enc.encode(arguments[i])
                }
            }

            return await revisor_LoadConstraints.apply(null, arguments)
        },
        validateDocument: async function(data) {
            if (typeof data == "string") {
                data = enc.encode(data)
            }

            const resultData = await revisor_ValidateDocument(data)

            return JSON.parse(dec.decode(resultData))
        }
    }

    if ('instantiateStreaming' in WebAssembly) {
        const obj = await WebAssembly.instantiateStreaming(fetch(WASM_URL), go.importObject)
        wasm = obj.instance
    } else {
        const resp = await fetch(WASM_URL)
        const obj = await WebAssembly.instantiate(resp.arrayBuffer(), go.importObject)

        wasm = obj.instance
    }

    go.run(wasm)

    const docArea = document.querySelector("[data-document]")
    const coreArea = document.querySelector("[data-core-constraints]")
    const resultArea = document.querySelector("[data-result]")

    const sampleDoc = await fetch("data/example-article.json")

    docArea.value = await sampleDoc.text()

    const docMirror = CodeMirror.fromTextArea(document.querySelector("[data-document]"), {
        lineNumbers: true,
        tabSize: 2,
        mode: {name: "javascript", json: true},
        theme: 'gruvbox-dark'
    })

    const coreConstraints = await fetch("data/naviga.json")

    coreArea.value = await coreConstraints.text()

    const coreConstrMirror = CodeMirror.fromTextArea(coreArea, {
        lineNumbers: true,
        tabSize: 2,
        mode: {name: "javascript", json: true},
        theme: 'gruvbox-dark'
    })

    const entityPath = function (err) {
        if (!err.entity) {
            return ["Document"]
        }

        let path = [];

        for (const e of err.entity.reverse()) {
            switch (e.refType) {
                case "block":
                    let qualifier = []

                    if (e.type) {
                        qualifier.push(`type="${e.type}"`)
                    }

                    if (e.rel) {
                        qualifier.push(`rel="${e.rel}"`)
                    }

                    const qString = qualifier.length ?
                        `{${qualifier.join(",")}}` : ""

                    const idx = e.index || 0

                    path.push(`${e.kind}[${idx}]${qString}`)
                    break
                case "data attribute":
                    path.push(`data.${e.name}`)
                    break
                case "attribute":
                    path.push(e.name)
            }
        }

        return path
    }

    const validateDoc = async function () {
        try {
            const result = await revisor.validateDocument(docMirror.getValue())

            resultArea.innerHTML = ""

            if (!result) {
                resultArea.innerText = "Valid document!"
                return
            }

            for (const r of result) {
                const path = entityPath(r)

                const eCont = document.createElement("div")
                eCont.classList.add("error")

                const ePath = document.createElement("div")
                ePath.classList.add("path")
                ePath.innerText = path.join(".")

                const eMessage = document.createElement("div")
                eMessage.classList.add("message")
                eMessage.innerText = r.error

                eCont.appendChild(ePath)
                eCont.appendChild(eMessage)
                resultArea.appendChild(eCont)
            }
        } catch (err) {
            resultArea.innerText = err.message
        }
    }

    const loadSchema = async function () {
        try {
            await revisor.loadConstraints(
                coreConstrMirror.getValue(),
            )
            await validateDoc()
        } catch (err) {
            resultArea.innerText = "Invalid constraints: " + err.message
        }
    }

    docMirror.on("change", validateDoc)
    coreConstrMirror.on("change", loadSchema)

    await loadSchema()
}

init();