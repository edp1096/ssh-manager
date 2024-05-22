function createUUID() {
    const getRandomValues = (length) => {
        const randomValues = new Uint8Array(length)
        crypto.getRandomValues(randomValues)
        return randomValues
    }
    const bytes = getRandomValues(16)

    let time = Date.now() * 10000 + 122192928000000000 // Unix epoch to UUID epoch

    // time low
    bytes[0] = (time & 0xff000000) >>> 24
    bytes[1] = (time & 0x00ff0000) >>> 16
    bytes[2] = (time & 0x0000ff00) >>> 8
    bytes[3] = (time & 0x000000ff);

    // time mid
    bytes[4] = (time & 0xff00000000) >>> 32
    bytes[5] = (time & 0x00ff000000) >>> 24

    // time high and version
    bytes[6] = ((time & 0x0f0000000000) >>> 40) | 0x10
    bytes[7] = (time & 0x00ff0000000000) >>> 48

    // clock_seq_hi_and_reserved
    bytes[8] = (bytes[8] & 0x3f) | 0x80

    // clock_seq_low and node
    const node = getRandomValues(6)
    for (let i = 10; i < 16; i++) {
        bytes[i] = node[i - 10]
    }

    const b2h = []
    for (let i = 0; i < 256; ++i) {
        b2h.push((i + 0x100).toString(16).substr(1))
    }

    const result = (
        b2h[bytes[0]] + b2h[bytes[1]] + b2h[bytes[2]] + b2h[bytes[3]] + '-' +
        b2h[bytes[4]] + b2h[bytes[5]] + '-' +
        b2h[bytes[6]] + b2h[bytes[7]] + '-' +
        b2h[bytes[8]] + b2h[bytes[9]] + '-' +
        b2h[bytes[10]] + b2h[bytes[11]] + b2h[bytes[12]] + b2h[bytes[13]] + b2h[bytes[14]] + b2h[bytes[15]]
    )

    return result
}
