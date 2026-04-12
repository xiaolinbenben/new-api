import { AppRoutes } from './app/routes';
import { WorkbenchProvider } from './features/workbench/context';

function App() {
  return (
    <WorkbenchProvider>
      <AppRoutes />
    </WorkbenchProvider>
  );
}

export default App;
