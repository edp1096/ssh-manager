const categoryEditDialog = document.querySelector('#dialog-category-edit')
const hostEditDialog = document.querySelector('#dialog-host-edit')
const categoryEditDialogTMPL = document.querySelector('#dialog-category-edit-template')
const hostEditDialogTMPL = document.querySelector('#dialog-host-edit-template')
const noticeDialogTMPL = document.querySelector('#dialog-notice-template')
const noticeDialog = document.querySelector('#dialog-notice')
const confirmDialogTMPL = document.querySelector('#dialog-confirm-template')
const confirmDialog = document.querySelector('#dialog-confirm')


async function getHosts() {
    const tmplHost = document.querySelector("#hosts-data-template").innerHTML
    const tmplCategoryButtons = document.querySelector("#category-buttons-template").innerHTML
    const tmplCategory = document.querySelector("#category-data-template").innerHTML
    const hostsContainer = document.querySelector("#hosts-data-container>.categories")

    const r = await fetch("/hosts?hosts-file=" + hostsFile)
    if (r.ok) {
        const response = await r.text()
        if (!response) {
            hostsData = []
            return
        }

        let isJSON = true
        try {
            JSON.parse(response)
        } catch (e) {
            isJSON = false
        }

        if (!isJSON) {
            alert(response)
            document.querySelector("#dialog-enter-password").showModal()
        }

        const json = JSON.parse(response)
        if (!json) { json = [] }

        hostsContainer.innerHTML = ""
        json["host-categories"].forEach((elCategory, i) => {
            let lines = ""
            if (elCategory["hosts"]) {
                elCategory["hosts"].forEach((elHosts, j) => {
                    let line = tmplHost
                    line = line.replaceAll("@@_NAME_@@", elHosts["name"])
                    line = line.replaceAll("@@_ADDRESS_@@", elHosts["address"])
                    line = line.replaceAll("@@_PORT_@@", elHosts["port"])
                    line = line.replaceAll("@@_CATEGORY_IDX_@@", (i + 1))
                    line = line.replaceAll("@@_HOST_IDX_@@", j + 1)

                    lines += line
                })
            }

            let cate = tmplCategory
            cate = cate.replaceAll("@@_CATEGORY_NAME_@@", elCategory["name"])
            cate = cate.replace("@@_CATEGORY_BUTTONS_@@", tmplCategoryButtons)
            cate = cate.replaceAll("@@_CATEGORY_IDX_@@", (i + 1))
            cate = cate.replaceAll("@@_HOST_DATA_@@", lines)

            hostsContainer.innerHTML += cate
        })

        hostsContainer.querySelectorAll('.category').forEach(item => {
            item.addEventListener('click', function (event) {
                if (!event.target.closest('.host-part-info') && !event.target.closest('button')) {
                    this.classList.toggle('active')
                }
            })
        })

        hostsData = json["host-categories"]
        return
    }
}

function expandAllCategories() {
    const cats = document.querySelector(".categories");
    for (const cat of cats.children) {
        if (!cat.classList.contains("active")) {
            cat.classList.add("active");
        }
    }
}

function collapseAllCategories() {
    const cats = document.querySelector(".categories");
    for (const cat of cats.children) {
        if (cat.classList.contains("active")) {
            cat.classList.remove("active");
        }
    }
}

function setAuthType() {
    const selectedAuthName = hostEditDialog.querySelector("input[name='auth-type']:checked").id.replaceAll("use-", "")

    const authTypes = hostEditDialog.querySelectorAll("input[name='auth-type']")
    for (const atype of authTypes) {
        const authName = atype.id.replaceAll("use-", "")
        const inputEL = hostEditDialog.querySelector(`[name='${authName}']`)

        inputEL.parentElement.style.display = "none"
        inputEL.removeAttribute("required")
    }

    const inputEL = hostEditDialog.querySelector(`[name='${selectedAuthName}']`)
    inputEL.parentElement.style.display = null

    inputEL.setAttribute("required", "")
    if (parseInt(hostEditDialog.querySelector("input#idx").value) > -1) {
        if (hostEditDialog.querySelector("input#auth-type-orig").value == selectedAuthName) {
            inputEL.removeAttribute("required")
        }
    }
}

function moveKeyFileToPrivateKeyText(el) {
    const file = el.target.files[0]
    const reader = new FileReader()
    reader.readAsText(file, 'UTF-8')
    reader.onload = (readerEvent) => {
        const content = readerEvent.target.result
        const d = hostEditDialog
        d.querySelector("textarea#host-edit-private-key-text").value = content
    }
}

function openCategoryEditDialog(categoryIdxSTR = null) {
    const tmpl = categoryEditDialogTMPL.innerHTML
    if (!categoryIdxSTR) {
        categoryEditDialog.innerHTML = tmpl.replaceAll("@@_TITLE_@@", "New category")
    } else {
        categoryEditDialog.innerHTML = tmpl.replaceAll("@@_TITLE_@@", "Edit category")
        const d = categoryEditDialog
        const idx = parseInt(categoryIdxSTR) - 1
        d.querySelector("input#category-idx").value = idx
        d.querySelector("input#category-name").value = hostsData[idx].name
    }

    categoryEditDialog.showModal()
}

async function saveCategoryData(e) {
    const d = e.target
    if (d.returnValue != 'confirm') {
        d.innerHTML = ""
        return
    }

    d.returnValue = ""

    let params = `hosts-file=${hostsFile}`

    const categoryIdxSTR = d.querySelector("input#category-idx").value
    if (categoryIdxSTR) {
        const idx = parseInt(categoryIdxSTR)
        if (idx > -1) {
            params += `&category-idx=${idx}`
        }
    }

    const categoryName = d.querySelector("input#category-name").value
    const categoryData = { name: categoryName }

    const r = await fetch(`/categories?${params}`, {
        method: "POST",
        body: JSON.stringify(categoryData)
    })

    if (r.ok) {
        const response = await r.json()
        if (response.message == "success") {
            message = "done to add/edit."
        }
    }

    hostEditDialog.innerHTML = ""
    const tmpl = noticeDialogTMPL.innerHTML
    noticeDialog.innerHTML = tmpl.replaceAll("@@_MESSAGE_@@", message)
    noticeDialog.showModal()
    getHosts()

    setTimeout(() => { noticeDialog.close() }, dialogCloseWaitTime)

    return
}

function cancelCategoryEditDialog() {
    categoryEditDialog.innerHTML = ""
    categoryEditDialog.close()
}

async function deleteCategory(idxSTR) {
    const idx = parseInt(idxSTR) - 1

    const params = `hosts-file=${hostsFile}&idx=${idx}`
    const r = await fetch(`/categories?${params}`, { method: "DELETE" })

    let message = "failed to delete."
    if (r.ok) {
        const response = await r.json()
        if (response.message == "success") {
            message = "done to delete."
        }
    }

    const tmpl = noticeDialogTMPL.innerHTML
    noticeDialog.innerHTML = tmpl.replaceAll("@@_MESSAGE_@@", message)
    noticeDialog.showModal()
    getHosts()

    setTimeout(() => { noticeDialog.close() }, dialogCloseWaitTime)

    return
}

function openHostEditDialog(categoryIdxSTR = null, hostIdxSTR = null) {
    const tmpl = hostEditDialogTMPL.innerHTML
    hostEditDialog.innerHTML = tmpl.replaceAll("@@_TITLE_@@", "New host")

    let categoryIdx = 0
    if (categoryIdxSTR) {
        categoryIdx = parseInt(categoryIdxSTR) - 1
    }

    hostEditDialog.querySelector("input#category-idx").value = categoryIdx

    if (hostIdxSTR) {
        const d = hostEditDialog
        const idx = parseInt(hostIdxSTR) - 1
        d.querySelector("input#idx").value = idx

        const hostItem = hostsData[categoryIdx].hosts[idx]
        const privateKeyText = (hostItem["private-key-text"]) ? hostItem["private-key-text"] : ""

        d.querySelector("input[name='name']").value = hostItem["name"]
        d.querySelector("input[name='address']").value = hostItem["address"]
        d.querySelector("input[name='port']").value = hostItem["port"]
        d.querySelector("input[name='username']").value = hostItem["username"]

        d.querySelector("input[name='password']").value = ""
        d.querySelector("input[name='password']").removeAttribute("required")

        d.querySelector("textarea[name='private-key-text']").value = privateKeyText
        d.querySelector("textarea[name='description']").value = hostItem["description"]

        d.querySelector("input#auth-type-orig").value = "password"
        if (privateKeyText != "") {
            d.querySelector("input#use-private-key-text").checked = true
            d.querySelector("input#auth-type-orig").value = "private-key-text"
        }
    }

    setAuthType()

    hostEditDialog.showModal()
}

async function saveHostData(e) {
    const d = e.target
    if (d.returnValue != 'confirm') {
        d.innerHTML = ""
        return
    }

    d.returnValue = ""

    const categoryIdxSTR = d.querySelector("input#category-idx").value
    const hostIdxSTR = d.querySelector("input#idx").value

    const name = d.querySelector("dialog input[name='name']").value
    const address = d.querySelector("dialog input[name='address']").value

    let port = d.querySelector("dialog input[name='port']").value.trim()
    if (!port) {
        port = 22
    }

    const username = d.querySelector("dialog input[name='username']").value

    let password, privateKeyText
    const selectedAuthName = hostEditDialog.querySelector("input[name='auth-type']:checked").id.replaceAll("use-", "")
    console.log(selectedAuthName)
    switch (selectedAuthName) {
        case "password":
            password = d.querySelector(`[name='${selectedAuthName}']`).value.trim()
            break
        case "private-key-text":
            privateKeyText = d.querySelector(`[name='${selectedAuthName}']`).value
            break
    }

    const description = d.querySelector("dialog textarea[name='description']").value

    const hostData = {
        name: name,
        address: address,
        port: parseInt(port),
        username: username,
        password: password,
        "private-key-text": privateKeyText,
        "description": description,
        "unique-id": createUUID()
    }

    let params = `hosts-file=${hostsFile}`

    if (categoryIdxSTR) {
        const categoryIdx = parseInt(categoryIdxSTR)
        if (categoryIdx > -1) {
            params += `&category-idx=${categoryIdx}`
        }
    }

    if (hostIdxSTR) {
        const idx = parseInt(hostIdxSTR)
        if (idx > -1) {
            params += `&host-idx=${idx}`
        }
    }

    const r = await fetch(`/hosts?${params}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(hostData)
    })

    let message = "failed to save."
    if (r.ok) {
        const response = await r.json()
        if (response.message == "success") {
            message = "done to save."
        }
    }

    hostEditDialog.innerHTML = ""
    const tmpl = noticeDialogTMPL.innerHTML
    noticeDialog.innerHTML = tmpl.replaceAll("@@_MESSAGE_@@", message)
    noticeDialog.showModal()
    getHosts()

    setTimeout(() => { noticeDialog.close() }, dialogCloseWaitTime)

    return
}

function cancelHostEditDialog() {
    hostEditDialog.innerHTML = ""
    hostEditDialog.close()
}

async function deleteHost(categoryIdxSTR, hostIdxSTR) {
    const categoryIDX = parseInt(categoryIdxSTR) - 1
    const hostIDX = parseInt(hostIdxSTR) - 1

    const params = `hosts-file=${hostsFile}&category-idx=${categoryIDX}&host-idx=${hostIDX}`
    const r = await fetch(`/hosts?${params}`, { method: "DELETE" })

    let message = "failed to delete."
    if (r.ok) {
        const response = await r.json()
        if (response.message == "success") {
            message = "done to delete."
        }
    }

    const tmpl = noticeDialogTMPL.innerHTML
    noticeDialog.innerHTML = tmpl.replaceAll("@@_MESSAGE_@@", message)
    noticeDialog.showModal()
    getHosts()

    setTimeout(() => { noticeDialog.close() }, dialogCloseWaitTime)

    return
}

async function showSavedPassword() {
    const d = hostEditDialog.querySelector("dialog input#host-edit-password")
    if (d.getAttribute("type") == "password") {
        d.setAttribute("type", "text")
    } else {
        d.setAttribute("type", "password")
    }
}

function closeNotice(e) {
    /* TODO: Close when click outside of dialog */
    // const target = e.target
    // const rect = target.getBoundingClientRect()
    // if (rect.left > e.clientX || rect.right < e.clientX ||
    //     rect.top > e.clientY || rect.bottom < e.clientY
    // ) {
    //     noticeDialog.close()
    // }

    noticeDialog.close()
}

function cancelConfirmDialog() {
    confirmDialog.innerHTML = ""
    confirmDialog.close()

    const datas = confirmDialogTMPL.content.querySelector("input[name='datas']")
    Object.keys(datas.dataset).forEach((k) => {
        delete datas.dataset[k]
    })
}

async function doSpecificJob(e) {
    const d = e.target
    if (d.returnValue != 'confirm') {
        d.innerHTML = ""
        return
    }

    d.returnValue = ""

    const jobType = confirmDialogTMPL.content.querySelector("input[name='job-type']").value
    const datas = confirmDialogTMPL.content.querySelector("input[name='datas']")

    switch (jobType) {
        case "delete-category":
            await deleteCategory(datas.dataset.categoryIdx)
            break
        case "delete-host":
            await deleteHost(datas.dataset.categoryIdx, datas.dataset.hostIdx)
            break
    }

    Object.keys(datas.dataset).forEach((k) => {
        delete datas.dataset[k]
    })
}

function openConfirm(message = "Press 'Ok' to proceed.") {
    confirmDialog.innerHTML = confirmDialogTMPL.innerHTML.replaceAll("@@_MESSAGE_@@", message)
    confirmDialog.showModal()
}

function openDeleteCategory(categoryIdxSTR) {
    confirmDialogTMPL.content.querySelector("input[name='job-type']").value = "delete-category"

    const datas = confirmDialogTMPL.content.querySelector("input[name='datas']")
    datas.dataset.categoryIdx = categoryIdxSTR

    openConfirm()
}

function openDeleteHost(categoryIdxSTR, hostIdxSTR) {
    confirmDialogTMPL.content.querySelector("input[name='job-type']").value = "delete-host"

    const datas = confirmDialogTMPL.content.querySelector("input[name='datas']")
    datas.dataset.categoryIdx = categoryIdxSTR
    datas.dataset.hostIdx = hostIdxSTR

    openConfirm()
}
