let orderData
let orderRequests

function createList() {
    const container = document.querySelector('#order-lists')
    container.innerHTML = ''
    orderData.forEach((item, idx) => {
        const itemDiv = createItem(item, idx)
        container.appendChild(itemDiv)
        if (!item.hosts) { return }
        item.hosts.forEach((subItem, subIdx) => {
            const subItemDiv = createSubItem(idx, subIdx, subItem)
            itemDiv.appendChild(subItemDiv)
        })
    })
}

function createItem(item, idx) {
    const itemDiv = document.createElement('div')
    itemDiv.className = 'item'
    itemDiv.draggable = true
    itemDiv.dataset.idx = idx
    itemDiv.innerHTML = item.name

    itemDiv.addEventListener('dragstart', handleDragStart)
    itemDiv.addEventListener('dragover', handleDragOver)
    itemDiv.addEventListener('drop', handleDrop)

    return itemDiv
}

function createSubItem(parentIdx, subIdx, subItem) {
    const subItemDiv = document.createElement('div')
    subItemDiv.className = 'sub-item'
    subItemDiv.draggable = true
    subItemDiv.dataset.parentIdx = parentIdx
    subItemDiv.dataset.idx = subIdx
    subItemDiv.innerHTML = subItem.name

    subItemDiv.addEventListener('dragstart', handleDragStart)
    subItemDiv.addEventListener('dragover', handleDragOver)
    subItemDiv.addEventListener('drop', handleDrop)

    return subItemDiv
}

function handleDragStart(e) {
    const itemIdx = parseInt(e.target.dataset.idx)
    const parentIdx = parseInt(e.target.dataset.parentIdx)

    e.dataTransfer.setData('itemIdx', itemIdx.toString())
    if (!isNaN(parentIdx)) {
        e.dataTransfer.setData('parentIdx', parentIdx.toString())
    }
}

function handleDragOver(e) {
    e.preventDefault()
    e.stopPropagation()
}

function handleDrop(e) {
    e.preventDefault()
    e.stopPropagation()

    const itemIdx = parseInt(e.dataTransfer.getData('itemIdx'))
    const parentIdx = parseInt(e.dataTransfer.getData('parentIdx'))
    const targetIdx = parseInt(e.target.dataset.idx)
    const targetParentIdx = parseInt(e.target.dataset.parentIdx)

    let draggedItem

    switch (true) {
        case (isNaN(parentIdx) && isNaN(targetParentIdx)):
            // item to item
            draggedItem = orderData[itemIdx]
            orderData.splice(itemIdx, 1)
            orderData.splice(targetIdx, 0, draggedItem)
            break
        case (isNaN(parentIdx) && !isNaN(targetParentIdx)):
            // item to sub-item
            draggedItem = orderData[itemIdx]
            orderData.splice(itemIdx, 1)
            orderData.splice(targetParentIdx, 0, draggedItem)
            break
        case (!isNaN(parentIdx) && isNaN(targetParentIdx)):
            // sub-item to item
            if (!orderData[targetIdx].hosts) {
                orderData[targetIdx].hosts = []
            }
            draggedItem = orderData[parentIdx].hosts[itemIdx]
            orderData[parentIdx].hosts.splice(itemIdx, 1)
            orderData[targetIdx].hosts.splice(targetIdx, 0, draggedItem)
            break
        case (!isNaN(parentIdx) && !isNaN(targetParentIdx)):
            // sub-item to sub-item
            draggedItem = orderData[parentIdx].hosts[itemIdx]
            orderData[parentIdx].hosts.splice(itemIdx, 1)
            orderData[targetParentIdx].hosts.splice(targetIdx, 0, draggedItem)
            break
    }

    createList()
}

function closeReorderMode() {
    orderData = []
    document.querySelector("#order-container").style.display = "none"
}

async function saveReorderedList() {
    const data = { "host-categories": orderData }

    const r = await fetch("/hosts?hosts-file=" + hostsFile, {
        method: "PATCH",
        body: JSON.stringify(data)
    })

    if (r.ok) {
        const response = await r.json()
        console.log(response)
    }

    getHosts()
    closeReorderMode()
}

function setReorderMode() {
    orderData = JSON.parse(JSON.stringify(hostsData))
    createList()

    const target = document.querySelector("body")
    target.removeEventListener('mousedown', preventDrag)

    document.querySelector("#order-container").style.display = "block"
}