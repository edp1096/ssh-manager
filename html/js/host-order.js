let orderData

function createList() {
    const container = document.getElementById('order-container')
    container.innerHTML = ''
    orderData.forEach((item, idx) => {
        const itemDiv = createItem(item, idx)
        container.appendChild(itemDiv)
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

    compareData()
    createList()
}

function compareData() {
    let changes = []
    let subChanges = []

    orderData.forEach((item, newIdx) => {
        newIdx = parseInt(newIdx)
        let originalIdx = hostsData.findIndex(originalItem => originalItem.name == item.name)
        if (originalIdx > -1 && originalIdx != newIdx) {
            changes.push({
                before: { idx: originalIdx, parentIdx: null },
                after: { idx: newIdx, parentIdx: null }
            })
        }

        for (const k in item.hosts) {
            const newSubIdx = parseInt(k)
            const subItem = item.hosts[newSubIdx]

            loopOrig:
            for (const i in hostsData) {
                const originalIdx = parseInt(i)
                const originalItem = hostsData[originalIdx]

                const originalSubIdx = originalItem.hosts.findIndex(originalSubItem => originalSubItem.name == subItem.name)
                if (originalSubIdx == -1) { continue }
                if (originalIdx == parseInt(newIdx)) {
                    if (originalSubIdx == newSubIdx) { continue }
                }

                for (const c of changes) {
                    if (c.before.idx == originalIdx && c.after.idx == newIdx) {
                        if (originalSubIdx == newSubIdx) { continue loopOrig }
                    }
                }

                subChanges.push({
                    before: { idx: originalSubIdx, parentIdx: originalIdx },
                    after: { idx: newSubIdx, parentIdx: newIdx }
                })
            }
        }
    })

    console.log("main:", changes)
    console.log("sub:", subChanges)
}

function setReorderMode() {
    orderData = JSON.parse(JSON.stringify(hostsData))
    createList()

    // document.querySelector("body").removeEventListener("mousedown", preventDrag)
    const target = document.querySelector("body")
    // const listenerList = getEventListeners(target)
    target.removeEventListener('mousedown', preventDrag)

    document.querySelector("#order-container").style.display = "block"
}