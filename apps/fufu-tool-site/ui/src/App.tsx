import { AdminPage } from './admin/AdminPage';
import { HomePage } from './HomePage';

export function App() {
  const pathname = window.location.pathname;
  if (pathname === '/admin' || pathname === '/admin/' || pathname === '/admin.html' || pathname === '/admin/index.html') {
    return <AdminPage />;
  }
  return <HomePage />;
}
