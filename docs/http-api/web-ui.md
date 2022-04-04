# Simple Web UI

You can try compiling TeX documents directly in your browser: Visit http://localhost:2201, and
you'll be greeted with a very basic, but functional UI.

Please note, that this UI is *not* built to work in every browser. It intentionally does not
use fancy build tools. It's just a simple HTML file, built by hand, using Bootstrap 5 for
aesthetics and Vue 3 for interaction. Both Bootstrap and Vue are bundled with texd, so you won't
need internet access for this to work.

If your browser does not support modern features like ES2022 proxies, `Object.entries`, `fetch`,
and `<object type="application/pdf" />` elements, you're out of luck. (Maybe upgrade your browser?)
Anyway, consider the UI only as demonstrator for the API.
