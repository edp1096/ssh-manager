let orderData
let orderPreviousFocus

function orderRow(item, idx, parentIdx) {
    const row = document.createElement('div'); row.className = 'order-row'
    const grip = document.createElement('span'); grip.className = 'order-grip'; grip.textContent = '⠿'; grip.setAttribute('aria-hidden', 'true')
    const name = document.createElement('span'); name.className = 'order-name'; name.textContent = item.name; name.title = item.name
    row.append(grip, name)
    const detail = document.createElement('span')
    detail.className = parentIdx === undefined ? 'order-count' : 'order-address'
    const count = item.hosts?.length || 0
    detail.textContent = parentIdx === undefined ? `${count} ${count === 1 ? 'host' : 'hosts'}` : `${item.address || ''}${item.port ? ':' + item.port : ''}`
    detail.title = detail.textContent; row.append(detail)
    const controls = document.createElement('span'); controls.className = 'order-move'
    const items = parentIdx === undefined ? orderData : orderData[parentIdx].hosts
    for (const [delta, label] of [[-1, 'Move up'], [1, 'Move down']]) {
        const button = document.createElement('button'); button.type = 'button'
        const icon = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
        icon.setAttribute('viewBox', '0 0 24 24'); icon.setAttribute('class', 'ui-icon'); icon.setAttribute('aria-hidden', 'true')
        const path = document.createElementNS('http://www.w3.org/2000/svg', 'path')
        path.setAttribute('d', delta < 0 ? 'M6 15l6-6 6 6' : 'M6 9l6 6 6-6'); icon.append(path); button.append(icon)
        button.title = label; button.setAttribute('aria-label', `${label}: ${item.name}`)
        button.disabled = idx + delta < 0 || idx + delta >= items.length
        button.addEventListener('click', () => {
            const target = idx + delta
            ;[items[idx], items[target]] = [items[target], items[idx]]
            createList()
            const selector = parentIdx === undefined ? `.item[data-idx="${target}"] > .order-row` : `.sub-item[data-parent-idx="${parentIdx}"][data-idx="${target}"] > .order-row`
            document.querySelector(`#order-lists ${selector} button:not(:disabled)`)?.focus()
        })
        controls.append(button)
    }
    row.append(controls)
    return row
}

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
    if (document.querySelector('#order-container').open) container.focus({ preventScroll: true })
}

function createItem(item, idx) {
    const itemDiv = document.createElement('div')
    itemDiv.className = 'item'
    itemDiv.draggable = true
    itemDiv.dataset.idx = idx
    itemDiv.append(orderRow(item, idx))

    itemDiv.addEventListener('dragstart', handleDragStart)
    itemDiv.addEventListener('dragover', handleDragOver)
    itemDiv.addEventListener('drop', handleDrop)
    itemDiv.addEventListener('dragend', clearDragStyles)
    itemDiv.addEventListener('dragleave', e => { if (!itemDiv.contains(e.relatedTarget)) itemDiv.classList.remove('drop-target') })

    return itemDiv
}

function createSubItem(parentIdx, subIdx, subItem) {
    const subItemDiv = document.createElement('div')
    subItemDiv.className = 'sub-item'
    subItemDiv.draggable = true
    subItemDiv.dataset.parentIdx = parentIdx
    subItemDiv.dataset.idx = subIdx
    subItemDiv.append(orderRow(subItem, subIdx, parentIdx))

    subItemDiv.addEventListener('dragstart', handleDragStart)
    subItemDiv.addEventListener('dragover', handleDragOver)
    subItemDiv.addEventListener('drop', handleDrop)
    subItemDiv.addEventListener('dragend', clearDragStyles)
    subItemDiv.addEventListener('dragleave', e => { if (!subItemDiv.contains(e.relatedTarget)) subItemDiv.classList.remove('drop-target') })

    return subItemDiv
}

function handleDragStart(e) {
    e.stopPropagation()
    if (e.target.closest('button')) { e.preventDefault(); return }
    const itemIdx = parseInt(e.currentTarget.dataset.idx)
    const parentIdx = parseInt(e.currentTarget.dataset.parentIdx)
    e.currentTarget.classList.add('dragging')
    e.dataTransfer.effectAllowed = 'move'

    e.dataTransfer.setData('itemIdx', itemIdx.toString())
    if (!isNaN(parentIdx)) {
        e.dataTransfer.setData('parentIdx', parentIdx.toString())
    }
}

function handleDragOver(e) {
    e.preventDefault()
    e.stopPropagation()
    document.querySelectorAll('#order-lists .drop-target').forEach(n => n.classList.remove('drop-target'))
    e.currentTarget.classList.add('drop-target')
    e.dataTransfer.dropEffect = 'move'
}

function clearDragStyles() { document.querySelectorAll('#order-lists .dragging, #order-lists .drop-target').forEach(n => n.classList.remove('dragging', 'drop-target')) }

function handleDrop(e) {
    e.preventDefault()
    e.stopPropagation()

    const itemIdx = parseInt(e.dataTransfer.getData('itemIdx'))
    const parentIdx = parseInt(e.dataTransfer.getData('parentIdx'))
    const targetIdx = parseInt(e.currentTarget.dataset.idx)
    const targetParentIdx = parseInt(e.currentTarget.dataset.parentIdx)
    if (!Number.isInteger(itemIdx) || !Number.isInteger(targetIdx) || !orderData[isNaN(parentIdx) ? itemIdx : parentIdx]) { clearDragStyles(); return }

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
            orderData[targetIdx].hosts.push(draggedItem)
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
    document.querySelector("#order-container").close()
    document.addEventListener('mousedown', preventDrag)
    if (orderPreviousFocus?.isConnected) orderPreviousFocus.focus()
}

async function saveReorderedList() {
    const data = { "host-categories": orderData }

    const save = document.querySelector('#order-buttons .ok')
    save.disabled = true
    try {
        const r = await fetch("/hosts?hosts-file=" + hostsFile, {
            method: "PATCH",
            body: JSON.stringify(data)
        })
        if (!r.ok) throw new Error('Could not save the new order. Please try again.')
        getHosts()
        closeReorderMode()
    } catch (error) { document.querySelector('#order-status').textContent = error.message }
    finally { save.disabled = false }
}

function setReorderMode() {
    orderData = JSON.parse(JSON.stringify(hostsData))
    createList()

    document.removeEventListener('mousedown', preventDrag)
    orderPreviousFocus = document.activeElement
    document.querySelector('#order-status').textContent = 'Changes are applied only when you save.'
    document.querySelector("#order-container").showModal()
}
document.querySelector('#order-container').addEventListener('cancel', event => { event.preventDefault(); closeReorderMode() })
document.querySelector('#order-container').addEventListener('keydown', event => {
    if (event.key === 'Escape' && !event.isComposing) { event.preventDefault(); event.stopPropagation(); closeReorderMode() }
})
