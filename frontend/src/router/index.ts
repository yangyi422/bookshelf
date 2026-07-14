import { createRouter, createWebHistory } from 'vue-router';
import { api } from '../api/client';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/setup', component: () => import('../pages/SetupPage.vue') },
    { path: '/login', component: () => import('../pages/LoginPage.vue') },
    {
      path: '/', component: () => import('../layouts/AppLayout.vue'), meta: { auth: true }, children: [
        { path: '', component: () => import('../pages/HomePage.vue') },
        { path: 'books/:id', component: () => import('../pages/BookDetailPage.vue') },
        { path: 'books/:id/edit', component: () => import('../pages/BookEditPage.vue') },
        { path: 'upload', component: () => import('../pages/UploadPage.vue') },
        { path: 'imports', component: () => import('../pages/ImportsPage.vue') },
        { path: 'catalog', component: () => import('../pages/CatalogPage.vue') },
        { path: 'trash', component: () => import('../pages/TrashPage.vue') },
        { path: 'system', component: () => import('../pages/SystemPage.vue') },
      ],
    },
  ],
});

router.beforeEach(async to => {
  try {
    const { data } = await api.get<{ initialized: boolean }>('/setup/status');
    if (!data.initialized) return to.path === '/setup' ? true : '/setup';
    if (to.path === '/setup') return '/login';
  } catch { return false; }
  if (!to.matched.some(record => record.meta.auth)) return true;
  try { await api.get('/auth/me'); return true; } catch { return '/login'; }
});

export default router;
