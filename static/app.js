(() => {
  'use strict';

  const { view, blog, query, myBlog } = window.MICHELANGELO;

  // ── State ──────────────────────────────────────────────────
  let offset = 0;
  let loading = false;
  let exhausted = false;
  let postType = 'photo';
  let posts = []; // all loaded posts for lightbox navigation
  let lightboxIndex = -1;

  // ── Elements ───────────────────────────────────────────────
  const grid       = document.getElementById('grid');
  const sentinel   = document.getElementById('sentinel');
  const loader     = document.getElementById('loader');
  const endMsg     = document.getElementById('end-message');
  const lightbox   = document.getElementById('lightbox');
  const lbMedia    = lightbox.querySelector('.lightbox-media');
  const lbClose    = lightbox.querySelector('.lightbox-close');
  const lbPrev     = lightbox.querySelector('.lightbox-prev');
  const lbNext     = lightbox.querySelector('.lightbox-next');
  const lbLike     = lightbox.querySelector('.btn-like');
  const lbReblog   = lightbox.querySelector('.btn-reblog');
  const lbSource   = lightbox.querySelector('.btn-source');

  // ── Type filter wiring ─────────────────────────────────────
  document.querySelectorAll('.type-filter a').forEach(a => {
    a.addEventListener('click', e => {
      e.preventDefault();
      if (a.dataset.type === postType) return;
      postType = a.dataset.type;
      document.querySelectorAll('.type-filter a').forEach(x => x.classList.remove('active'));
      a.classList.add('active');
      reset();
    });
  });

  // ── API fetch ──────────────────────────────────────────────
  async function fetchPosts() {
    if (loading || exhausted) return;
    loading = true;
    loader.classList.remove('hidden');

    let url;
    if (view === 'dashboard') {
      url = `/api/dashboard?offset=${offset}&type=${postType}`;
    } else if (view === 'blog') {
      url = `/api/blog/${encodeURIComponent(blog)}?offset=${offset}&type=${postType}`;
    } else if (view === 'search') {
      url = `/api/search?q=${encodeURIComponent(query)}`;
      exhausted = true; // tagged endpoint returns one batch
    }

    try {
      const resp = await fetch(url);
      if (!resp.ok) throw new Error(await resp.text());
      const data = await resp.json();
      const newPosts = Array.isArray(data) ? data : (data || []);

      if (!newPosts.length) {
        exhausted = true;
        endMsg.classList.remove('hidden');
      } else {
        renderPosts(newPosts);
        offset += newPosts.length;
        if (newPosts.length < 20) {
          exhausted = true;
          endMsg.classList.remove('hidden');
        }
      }
    } catch (err) {
      console.error('Fetch error:', err);
    } finally {
      loading = false;
      loader.classList.add('hidden');
    }
  }

  // ── Render ─────────────────────────────────────────────────
  function renderPosts(newPosts) {
    const startIndex = posts.length;
    posts = posts.concat(newPosts);

    newPosts.forEach((post, i) => {
      const idx = startIndex + i;
      const card = buildCard(post, idx);
      if (card) grid.appendChild(card);
    });
  }

  function buildCard(post, idx) {
    let mediaSrc = null;
    let isVideo = false;
    let thumb = null;

    if (post.type === 'photo' && post.photos && post.photos.length) {
      mediaSrc = post.photos[0].original_size.url;
    } else if (post.type === 'video' && post.video_url) {
      mediaSrc = post.video_url;
      thumb = post.thumbnail_url;
      isVideo = true;
    }

    if (!mediaSrc) return null;

    const card = document.createElement('div');
    card.className = 'post-card';
    card.dataset.index = idx;

    if (isVideo) {
      const vid = document.createElement('video');
      vid.src = mediaSrc;
      if (thumb) vid.poster = thumb;
      vid.muted = true;
      vid.loop = true;
      vid.playsInline = true;
      vid.addEventListener('mouseenter', () => vid.play());
      vid.addEventListener('mouseleave', () => { vid.pause(); vid.currentTime = 0; });
      card.appendChild(vid);
    } else {
      const img = document.createElement('img');
      img.src = mediaSrc;
      img.loading = 'lazy';
      img.alt = '';
      card.appendChild(img);
    }

    // Hover meta bar
    const meta = document.createElement('div');
    meta.className = 'post-meta';
    meta.innerHTML = `
      <span class="blog-name">${esc(post.blog_name)}</span>
      <span class="note-count">${fmtNotes(post.note_count)}</span>
    `;
    card.appendChild(meta);

    card.addEventListener('click', () => openLightbox(idx));
    return card;
  }

  // ── Lightbox ───────────────────────────────────────────────
  function openLightbox(idx) {
    lightboxIndex = idx;
    renderLightbox();
    lightbox.classList.remove('hidden');
    document.body.style.overflow = 'hidden';
  }

  function closeLightbox() {
    lightbox.classList.add('hidden');
    document.body.style.overflow = '';
    lbMedia.innerHTML = '';
  }

  function renderLightbox() {
    const post = posts[lightboxIndex];
    if (!post) return;

    lbMedia.innerHTML = '';
    lbPrev.style.visibility = lightboxIndex > 0 ? 'visible' : 'hidden';
    lbNext.style.visibility = lightboxIndex < posts.length - 1 ? 'visible' : 'hidden';

    if (post.type === 'photo' && post.photos && post.photos.length) {
      const img = document.createElement('img');
      img.src = post.photos[0].original_size.url;
      img.alt = '';
      lbMedia.appendChild(img);
    } else if (post.type === 'video' && post.video_url) {
      const vid = document.createElement('video');
      vid.src = post.video_url;
      if (post.thumbnail_url) vid.poster = post.thumbnail_url;
      vid.controls = true;
      vid.autoplay = true;
      lbMedia.appendChild(vid);
    }

    // Actions
    lbLike.dataset.id   = post.id_string || post.id;
    lbLike.dataset.key  = post.reblog_key;
    lbLike.dataset.liked = post.liked ? 'true' : 'false';
    lbLike.classList.toggle('liked', !!post.liked);
    lbLike.textContent = post.liked ? '♥ Liked' : '♡ Like';

    lbReblog.dataset.id   = post.id_string || post.id;
    lbReblog.dataset.key  = post.reblog_key;
    lbReblog.dataset.blog = post.blog_name;

    lbSource.href = post.post_url || '#';

    // Prefetch next batch if near end
    if (lightboxIndex >= posts.length - 5) {
      fetchPosts();
    }
  }

  lbClose.addEventListener('click', closeLightbox);
  lightbox.addEventListener('click', e => { if (e.target === lightbox) closeLightbox(); });

  lbPrev.addEventListener('click', () => {
    if (lightboxIndex > 0) { lightboxIndex--; renderLightbox(); }
  });
  lbNext.addEventListener('click', () => {
    if (lightboxIndex < posts.length - 1) { lightboxIndex++; renderLightbox(); }
  });

  document.addEventListener('keydown', e => {
    if (lightbox.classList.contains('hidden')) return;
    if (e.key === 'Escape')       closeLightbox();
    if (e.key === 'ArrowLeft')    lbPrev.click();
    if (e.key === 'ArrowRight')   lbNext.click();
  });

  // ── Like ───────────────────────────────────────────────────
  lbLike.addEventListener('click', async () => {
    const id   = lbLike.dataset.id;
    const key  = lbLike.dataset.key;
    const liked = lbLike.dataset.liked === 'true';
    const url  = `/api/like?id=${encodeURIComponent(id)}&key=${encodeURIComponent(key)}${liked ? '&unlike=1' : ''}`;
    try {
      const resp = await fetch(url, { method: 'GET' });
      if (!resp.ok) throw new Error(await resp.text());
      const post = posts[lightboxIndex];
      post.liked = !liked;
      lbLike.dataset.liked = post.liked ? 'true' : 'false';
      lbLike.classList.toggle('liked', post.liked);
      lbLike.textContent = post.liked ? '♥ Liked' : '♡ Like';
    } catch (err) {
      console.error('Like error:', err);
    }
  });

  // ── Reblog ─────────────────────────────────────────────────
  lbReblog.addEventListener('click', async () => {
    if (!myBlog) return;
    const body = new URLSearchParams({
      id:         lbReblog.dataset.id,
      reblog_key: lbReblog.dataset.key,
      blog_name:  lbReblog.dataset.blog,
      native_blog: myBlog,
    });
    try {
      const resp = await fetch('/api/reblog', { method: 'POST', body });
      if (!resp.ok) throw new Error(await resp.text());
      lbReblog.textContent = '✓ Reblogged';
      setTimeout(() => { lbReblog.textContent = '⇄ Reblog'; }, 2000);
    } catch (err) {
      console.error('Reblog error:', err);
    }
  });

  // ── Infinite scroll via IntersectionObserver ───────────────
  const observer = new IntersectionObserver(entries => {
    if (entries[0].isIntersecting) fetchPosts();
  }, { rootMargin: '400px' });

  observer.observe(sentinel);

  // ── Reset (type filter change) ─────────────────────────────
  function reset() {
    offset = 0;
    loading = false;
    exhausted = false;
    posts = [];
    lightboxIndex = -1;
    grid.innerHTML = '';
    endMsg.classList.add('hidden');
    fetchPosts();
  }

  // ── Blog link interception ─────────────────────────────────
  // Clicking a blog name in the meta overlay navigates to /blog/<name>
  grid.addEventListener('click', e => {
    const blogLink = e.target.closest('.blog-name');
    if (blogLink && blogLink.dataset.name) {
      e.stopPropagation();
      window.location.href = `/blog/${encodeURIComponent(blogLink.dataset.name)}`;
    }
  });

  // ── Helpers ────────────────────────────────────────────────
  function esc(str) {
    return String(str || '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  function fmtNotes(n) {
    if (!n) return '';
    if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
    return String(n);
  }

  // ── Init ───────────────────────────────────────────────────
  fetchPosts();

})();
