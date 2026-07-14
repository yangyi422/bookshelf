import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { createRouter, createMemoryHistory } from 'vue-router';
import ElementPlus from 'element-plus';
import { api } from '../api/client';
import LoginPage from '../pages/LoginPage.vue';
import HomePage from '../pages/HomePage.vue';
import UploadPage from '../pages/UploadPage.vue';
import BookDetailPage from '../pages/BookDetailPage.vue';
import SetupPage from '../pages/SetupPage.vue';

const tick = () => new Promise(resolve => setTimeout(resolve, 0));
const longFilename = '这是一个用于验证详情页省略显示但仍然可以查看完整内容的超长电子书文件名称.epub';

function testRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/login', component: { template: '<div />' } },
      { path: '/upload', component: { template: '<div />' } },
      { path: '/books/:id', component: { template: '<div />' } },
    ],
  });
}

afterEach(() => vi.restoreAllMocks());

describe('core pages', () => {
  it('shows secure OPDS defaults in the initialization wizard', async () => {
    const router = testRouter();
    const wrapper = mount(SetupPage, { global: { plugins: [router, ElementPlus] } });
    expect(wrapper.text()).toContain('仅 HTTPS（推荐）');
    expect(wrapper.text()).toContain('OPDS 密码');
    expect((wrapper.find('input[type=password]').element as HTMLInputElement).value).toBe('');
  });
  it('submits login credentials', async () => {
    const post = vi.spyOn(api, 'post').mockResolvedValue({ data: {} } as any);
    const router = testRouter();
    await router.push('/login');
    await router.isReady();
    const wrapper = mount(LoginPage, { global: { plugins: [router, ElementPlus] } });
    const inputs = wrapper.findAll('input');
    await inputs[0].setValue('admin');
    await inputs[1].setValue('correct horse battery staple');
    await wrapper.find('form').trigger('submit');
    await tick();
    expect(post).toHaveBeenCalledWith('/auth/login', { username: 'admin', password: 'correct horse battery staple' });
  });

  it('renders books and explicit filter placeholders', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        items: [{ id: 'b1', title: '测试书籍', subtitle: '', description: '', language: 'zh', publisher: '', isbn: '', cover_path: '', reading_status: 'unread', rating: 0, created_at: '', files: [], authors: [], tags: [] }],
        total: 1,
        page: 1,
        page_size: 20,
      },
    } as any);
    const router = testRouter();
    await router.push('/');
    await router.isReady();
    const wrapper = mount(HomePage, { global: { plugins: [router, ElementPlus] } });
    await tick();
    expect(wrapper.text()).toContain('测试书籍');
    expect(wrapper.text()).toContain('文件格式');
    expect(wrapper.text()).toContain('阅读状态');
  });

  it('renders a Chinese reading status and preserves the full long filename in title', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        id: 'b1', title: '测试书籍', subtitle: '', description: '', language: 'zh', publisher: '', isbn: '', cover_path: '', reading_status: 'reading', rating: 0, created_at: '', authors: [], tags: [],
        files: [{ id: 'f1', book_id: 'b1', format: 'epub', mime_type: 'application/epub+zip', original_name: longFilename, file_size: 1024, sha256: 'abc' }],
      },
    } as any);
    const router = testRouter();
    await router.push('/books/b1');
    await router.isReady();
    const wrapper = mount(BookDetailPage, { global: { plugins: [router, ElementPlus] } });
    await tick();
    expect(wrapper.text()).toContain('在读');
    expect(wrapper.find('.file-name').attributes('title')).toBe(longFilename);
  });

  it('uploads the selected file', async () => {
    const post = vi.spyOn(api, 'post').mockResolvedValue({ data: { book: { id: 'b1' } } } as any);
    const router = testRouter();
    await router.push('/upload');
    await router.isReady();
    const wrapper = mount(UploadPage, { global: { plugins: [router, ElementPlus] } });
    const input = wrapper.find('input[type=file]');
    const file = new File(['%PDF-'], 'book.pdf', { type: 'application/pdf' });
    Object.defineProperty(input.element, 'files', { value: [file] });
    await input.trigger('change');
    await wrapper.find('button').trigger('click');
    await tick();
    expect(post).toHaveBeenCalled();
    expect((post.mock.calls[0][1] as FormData).get('file')).toBe(file);
  });
});
