import { AppRouter } from './router';
import { WorkbenchProvider } from './features/workbench/context';

function App() {
  return (
    <WorkbenchProvider>
      <AppRouter />
    </WorkbenchProvider>
  );
}

export default App;
