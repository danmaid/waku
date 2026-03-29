/**
 * <dm-honeycomb> カスタムエレメント
 * - アイテムをハニカムレイアウトで表示するためのコンテナ
 * - アイテムは `items` プロパティで与える（配列）
 * - レイアウトはビューポートサイズとアイテム数に応じて自動調整される
 * - アイテムの見た目は、テンプレート要素を用いてカスタマイズ可能
 *   （例：<template><div class="cell item-cell"></div></template>）
 * - アイテムは、レイアウト内のセルに順番に配置される
 * - セルは、CSS変数 `--cx` と `--cy` を用いて位置が指定される
 * - セルのサイズは、CSS変数 `--hex-width` と `--hex-height` で指定される
 * - レイアウトは、アイテム数とビューポートサイズに基づいて、列数と行数を計算することで決定される
 * - 列数の計算は、二次方程式を解析的に解くことで行われる
 * - レイアウトは、ResizeObserver を用いてビューポートサイズの変化に対応する
 * - アイテムの配置は、ビューポート内に収まるセルに対して行われる
 * - アイテムは、セルのデータ属性としても利用可能（例：data-name="item1"）
 * - アイテムの見た目は、CSSクラスやデータ属性を用いてカスタマイズ可能
 * - 例：アイテムに `critical` プロパティがある場合、セルに `critical` クラスを追加するなど
 * - アイテムの更新は、`items` プロパティのセッターで行われる
 * - レイアウトの更新は、`updateLayout` メソッドで行われる
 * - レイアウトの更新は、アイテム数やビューポートサイズの変化に応じて呼び出される
 * - アイテムの配置は、レイアウト内のセルに順番に行われるが、ビューポート内に収まるセルに対してのみ行われる
 * - アイテムの配置は、セルの位置（--cx, --cy）とサイズ（--hex-width, --hex-height）を用いて行われる
 * - アイテムの見た目は、CSSクラスやデータ属性を用いてカスタマイズ可能
 */
customElements.define('dm-honeycomb', class DmHoneycomb extends HTMLElement {
  #template
  /** @type {Array|null} */
  #items
  get items() { return this.#items }
  set items(value) {
    this.#items = value
    this.updateLayout()
  }

  constructor() {
    super()
    // クリッピング用の SVG 定義を追加
    const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg")
    svg.setAttribute("width", "0")
    svg.setAttribute("height", "0")
    svg.style.position = "absolute"
    svg.innerHTML = `
      <defs>
        <clipPath id="dm-hex-mask" clipPathUnits="objectBoundingBox">
          <polygon points="0.5 0, 1 0.25, 1 0.75, 0.5 1, 0 0.75, 0 0.25"/>
        </clipPath>
      </defs>
      `
    document.body.appendChild(svg)
    // テンプレート
    this.#template = document.createElement('template')
    this.#template.innerHTML = '<div></div>'
  }

  connectedCallback() {
    const style = document.createElement('style')
    style.innerHTML = `
        dm-honeycomb {
          display: block;
          position: relative;
          width: 100%;
          height: 100%;
          overflow: hidden;
        }
        dm-honeycomb .cell {
          position: absolute;
          width: var(--hex-width);
          height: var(--hex-height);
          background: #555;
          clip-path: url(#dm-hex-mask);
          transform: translate(var(--cx), var(--cy));
        }
        dm-honeycomb .cell.item-cell {
          background: #4170f1;
        }
      `
    this.appendChild(style)
    requestAnimationFrame(() => {
      const template = this.querySelector('template')
      if (template) this.#template = template
      new ResizeObserver(() => this.updateLayout()).observe(this)
    })
  }

  async updateLayout() {
    const size = this.#items?.length
    if (!size) return
    const x = this.computeHexLayout(this.clientWidth, this.clientHeight, size)
    console.log('Computed layout', x)
    const { columns, rows, itemWidth, itemHeight, bottomPadding } = x
    this.style.setProperty('--hex-width', `${itemWidth}px`)
    this.style.setProperty('--hex-height', `${itemHeight * (4 / 3)}px`)

    for (const em of this.querySelectorAll('.cell')) em.remove()
    let i = 0
    for (let row = -1; row * itemHeight < this.clientHeight; row++) {
      const isEvenRow = ((row + 1) % 2) === 0
      for (let col = isEvenRow ? -1 : 0; col < columns; col++) {
        const cx = col * itemWidth + (isEvenRow ? itemWidth / 2 : 0)
        const cy = row * itemHeight
        const em = this.#template.content.firstElementChild.cloneNode(true)
        em.classList.add('cell')
        em.style.setProperty('--cx', `${cx}px`)
        em.style.setProperty('--cy', `${cy}px`)
        if (cx >= 0 && cy >= 0 && cx + itemWidth <= this.clientWidth && cy + itemHeight <= this.clientHeight) {
          const item = this.#items[i]
          if (item) {
            em.item = item
            em.classList.add('item-cell')
            for (const [k, v] of Object.entries(item)) em.dataset[k] = v
            i++
          }
        }
        this.appendChild(em)
      }
    }
  }

  // ** @type {(width: number, height: number, items: number) => {columns: number, rows: number, itemWidth: number, itemHeight: number, bottomPadding: number}} */
  computeHexLayout(width, height, items) {
    const ar = Math.sqrt(3) / 2 // レイアウト用 pointy-top hex の縦/横比
    const r = this.computeGridLayout(width, height, items, ar)
    const missing = Math.floor(r.rows / 2) * 2 // 半ずれ欠損分
    if (r.rows * r.columns - missing >= items) return r
    // 不足時の再計算
    return this.computeGridLayout(width, height, items + missing, ar)
  }

  // ** @type {(viewportWidth: number, viewportHeight: number, numberOfItems: number, aspectRatio: number)
  //  => {columns: number, rows: number, itemWidth: number, itemHeight: number, bottomPadding: number}} */
  computeGridLayout(width, height, items, aspectRatio) {
    // ================================
    // 列数決定（試行なし・解析的）
    // ================================
    //
    // 列数 C を連続量として考えたとき、
    // 「縦に必要な高さ == height」になる
    // 境界の C を二次方程式として解く。
    //
    // ※ 列数についての二次方程式を「解の公式」で解いた結果
    // 二次方程式 ax^2 + bx + c = 0 の係数
    // a: 縦方向の制限
    // b: 横幅起因の一次項（符号はマイナス）
    // c: 最悪ケースとして (items - 1) 個ぶんが
    //    縦に余計に積まれる影響
    const minimumColumnsNeeded = this.calcKainoKoushikiPositiveRoot(
      height,
      -(aspectRatio * width),
      -(aspectRatio * width) * (items - 1))

    // 実際に使う列数（不足は致命的なので切り上げ）
    const columns = Math.ceil(minimumColumnsNeeded)

    // ================================
    // サイズ・行数の確定
    // ================================

    // 横は必ず使い切る
    const itemWidth = width / columns

    // アスペクト比固定
    const itemHeight = itemWidth * aspectRatio

    // 必要な行数
    const rows = Math.ceil(items / columns)

    // 下に余る高さ（レイアウト調整用）
    const bottomPadding = height - rows * itemHeight

    return { columns, rows, itemWidth, itemHeight, bottomPadding }
  }

  calcKainoKoushikiPositiveRoot(a, b, c) {
    // --------------------------------------------------
    // 解の公式（Quadratic Formula）
    //   ax^2 + bx + c = 0
    // の解は：
    //   x = (-b ± √(b^2 - 4ac)) / (2a)
    // この関数では、
    // ・x が「列数」を表す
    // ・列数は正である必要がある
    //
    // という理由から、
    //   「+ √(...) 側の解」
    // のみを返す。
    // --------------------------------------------------

    // 正の解（境界点）
    return (-b + Math.sqrt(b ** 2 - 4 * a * c)) / (2 * a)
  }

  // ================================
  // 列数決定（試行あり・シミュレーション）
  // ================================
  // 列数を 1 から増やしながら、
  // 「その列数で並べたときの全体の高さ」が
  // ビューポートの高さを超えるかどうかを試す。
  // 
  // **** ボツ ****
  // const columns = findColumnsByTrial({ viewportWidth, viewportHeight, numberOfItems, aspectRatio })
  // で使えるっちゃ使えるが、計算量が多い（特に列数が多いとき）ので、解析的に求める方法を採用した。
  findColumnsByTrial({ viewportWidth, viewportHeight, numberOfItems, aspectRatio }) {
    for (let columns = 1; columns <= numberOfItems; columns++) {
      const itemWidth = viewportWidth / columns
      const itemHeight = itemWidth * aspectRatio
      const rows = Math.ceil(numberOfItems / columns)
      const totalHeight = rows * itemHeight

      if (totalHeight <= viewportHeight) {
        return columns
      }
    }
    throw new Error("入る列数が見つからない")
  }
})
