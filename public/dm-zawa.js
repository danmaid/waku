// dm-zawa.js (count-only mapping: equal-interval timing, constant life, single "ざわ…")
class DmZawa extends HTMLElement {
  static get observedAttributes() {
    return [
      'value',           // 0..100 -> events per window
      'window-ms',       // window length (ms), default 5000
      'min-per-window',  // minimal events per window (default 1)
      'life',            // ms (default 2000)
      'max-overlap',     // simultaneous limit
      'bleed',           // 0..1
      'color','stroke','font-size','scale'
    ];
  }
  get value() { return Number(this.getAttribute('value') ?? 0); }
  set value(v) { this.setAttribute('value', String(v)); }

  constructor() {
    super();
    this._root = this.attachShadow({ mode: 'open' });
    this._timer = null;
    this._running = false;
    this._bbox = { w: 0, h: 0 };
    this._currentOverlays = 0;
    this._interval = 1000;
    this._root.innerHTML = `
      <style>
        :host { display: inline-block; position: relative; }
        .wrap { position: relative; display: inline-grid; }
        .overlay { pointer-events: none; position: absolute; inset: 0; overflow: visible; }
        svg { position: absolute; will-change: transform, opacity; filter: drop-shadow(0 0 1px rgba(0,0,0,.25)); overflow: visible; }
      </style>
      <div class="wrap">
        <slot></slot>
        <div class="overlay" aria-hidden="true"></div>
      </div>`;
    this.$overlay = this._root.querySelector('.overlay');

    this._resizeObs = new ResizeObserver(entries => {
      for (const e of entries) {
        const cr = e.contentRect;
        this._bbox = { w: cr.width, h: cr.height };
      }
    });
    this._intersectObs = new IntersectionObserver(entries => {
      const on = entries.some(e => e.isIntersecting);
      if (on) this._start(); else this._stop();
    }, { threshold: 0.01 });
  }

  connectedCallback() {
    const slotEl = this._root.querySelector('slot');
    const updateTargets = () => { for (const n of slotEl.assignedElements()) this._resizeObs.observe(n); };
    slotEl.addEventListener('slotchange', updateTargets);
    updateTargets();
    this._intersectObs.observe(this);
    this._recalcSchedule();
    this._start();
  }
  disconnectedCallback() {
    this._stop(); this._resizeObs.disconnect(); this._intersectObs.disconnect();
  }
  attributeChangedCallback(name) {
    if (['value','window-ms','min-per-window','max-overlap','life'].includes(name)) {
      this._recalcSchedule();
    }
  }

  _start() { if (this._running) return; this._running = true; this._setTimer(); }
  _stop()  { this._running = false; clearInterval(this._timer); this._timer = null; }
  _setTimer() { clearInterval(this._timer); if (!this._running) return; this._timer = setInterval(() => this._tick(), this._interval); }

  _paramNum(name, def, min=-Infinity, max=Infinity) {
    const v = Number(this.getAttribute(name));
    if (Number.isFinite(v)) return Math.min(max, Math.max(min, v));
    return def;
  }

  _recalcSchedule() {
    // Compute equal-interval based solely on value -> events per window
    const v = this._paramNum('value', 0, 0, 100);
    const windowMs = this._paramNum('window-ms', 5000, 100, 600000);
    const minPer = this._paramNum('min-per-window', 1, 0, 1000);
    let perWindow = Math.round(v);
    if (perWindow < minPer) perWindow = minPer;
    if (perWindow <= 0) perWindow = 1; // avoid division by zero
    this._interval = Math.max(10, windowMs / perWindow);

    const maxOverlap = this._paramNum('max-overlap', 64, 0, 1024);
    this._concurrencyCap = maxOverlap;

    if (this._running) this._setTimer();
  }

  _tick() {
    if (!this._running) return;
    if (this._bbox.w <= 0 || this._bbox.h <= 0) return;
    if (this._currentOverlays >= this._concurrencyCap) return; // cap reached -> skip
    this._spawnOne();
  }

  _spawnOne() {
    const color = this.getAttribute('color') ?? '#ffffff';
    const stroke = this.getAttribute('stroke') ?? '#000000';
    const scale = this._paramNum('scale', 1.0, 0.5, 3.0);
    const fs = this._paramNum('font-size', 18, 10, 64);
    const texts = ['ざわ…'];
    const text = texts[0];

    // Random position only (systematic timing)
    const margin = Math.max(8, fs*0.6);
    const bleed = this._paramNum('bleed', 0, 0, 1);
    const w = this._bbox.w, h = this._bbox.h;
    const xMin = -bleed * w + margin;
    const xMax = (1 + bleed) * w - margin;
    const yMin = -bleed * h + margin;
    const yMax = (1 + bleed) * h - margin;
    const x = xMin + Math.random() * Math.max(1, xMax - xMin);
    const y = yMin + Math.random() * Math.max(1, yMax - yMin);

    const svg = document.createElementNS('http://www.w3.org/2000/svg','svg');
    svg.setAttribute('width', '1'); svg.setAttribute('height','1'); svg.setAttribute('overflow','visible');
    svg.style.left = `${x}px`; svg.style.top  = `${y}px`;

    const g = document.createElementNS('http://www.w3.org/2000/svg','g');
    g.setAttribute('transform', `translate(${-0.5},${-0.5}) scale(${scale})`);
    const t = document.createElementNS('http://www.w3.org/2000/svg','text');
    t.textContent = text;
    t.setAttribute('x','0'); t.setAttribute('y','0');
    t.setAttribute('dominant-baseline','middle'); t.setAttribute('text-anchor','middle');
    t.setAttribute('font-size', `${fs}`);
    t.setAttribute('font-family', `system-ui, "Hiragino Kaku Gothic ProN", Meiryo, sans-serif`);
    t.setAttribute('fill', color); t.setAttribute('stroke', stroke); t.setAttribute('stroke-width', '0.8');
    g.appendChild(t); svg.appendChild(g); this.$overlay.appendChild(svg);

    // Constant life (default 2000ms)
    const life = this._paramNum('life', 2000, 100, 60000);
    const anim = svg.animate([
      { transform: `translate(0,0) rotate(0deg) scale(1)`,   opacity: 0 },
      { transform: `translate(0,0) rotate(0deg) scale(1.03)`, opacity: 1, offset: 0.2 },
      { transform: `translate(0,0) rotate(0deg) scale(1.03)`, opacity: 1, offset: 0.7 },
      { transform: `translate(0,0) rotate(0deg) scale(0.95)`, opacity: 0 }
    ], { duration: life, easing: 'ease-out', fill: 'forwards' });

    this._currentOverlays++;
    anim.onfinish = anim.oncancel = () => { svg.remove(); this._currentOverlays = Math.max(0, this._currentOverlays - 1); };
  }
}
customElements.define('dm-zawa', DmZawa);
