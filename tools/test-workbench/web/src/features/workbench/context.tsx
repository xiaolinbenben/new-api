import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { Toast } from '@douyinfe/semi-ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useLocation } from 'react-router-dom';
import { deleteJSON, getJSON, postJSON, putJSON } from '../../lib/api';
import type {
  Environment,
  EventsResponse,
  LoadRunRuntime,
  MockListenerRuntime,
  MockProfile,
  Project,
  RouteStatsItem,
  RunListItem,
  RunProfile,
  RunRecord,
  Scenario
} from '../../types';
import {
  cloneValue,
  createDefaultMockProfileConfig,
  createDefaultRunProfileConfig,
  createDefaultScenarioConfig
} from './helpers';

function useWorkbenchState() {
  const queryClient = useQueryClient();
  const location = useLocation();
  const [selectedProjectId, setSelectedProjectId] = useState('');
  const [selectedEnvironmentId, setSelectedEnvironmentId] = useState('');
  const [selectedMockProfileId, setSelectedMockProfileId] = useState('');
  const [selectedRunProfileId, setSelectedRunProfileId] = useState('');
  const [selectedScenarioId, setSelectedScenarioId] = useState('');
  const [selectedRunId, setSelectedRunId] = useState('');

  const [projectName, setProjectName] = useState('');
  const [projectDescription, setProjectDescription] = useState('');
  const [environmentDraft, setEnvironmentDraft] = useState<Environment | null>(null);
  const [mockProfileDraft, setMockProfileDraft] = useState<MockProfile | null>(null);
  const [runProfileDraft, setRunProfileDraft] = useState<RunProfile | null>(null);
  const [scenarioDraft, setScenarioDraft] = useState<Scenario | null>(null);

  const projectsQuery = useQuery({
    queryKey: ['projects'],
    queryFn: () => getJSON<Project[]>('/projects')
  });

  const environmentsQuery = useQuery({
    queryKey: ['projects', selectedProjectId, 'environments'],
    queryFn: () => getJSON<Environment[]>(`/projects/${selectedProjectId}/environments`),
    enabled: Boolean(selectedProjectId)
  });

  const mockProfilesQuery = useQuery({
    queryKey: ['projects', selectedProjectId, 'mock-profiles'],
    queryFn: () => getJSON<MockProfile[]>(`/projects/${selectedProjectId}/mock-profiles`),
    enabled: Boolean(selectedProjectId)
  });

  const runProfilesQuery = useQuery({
    queryKey: ['projects', selectedProjectId, 'run-profiles'],
    queryFn: () => getJSON<RunProfile[]>(`/projects/${selectedProjectId}/run-profiles`),
    enabled: Boolean(selectedProjectId)
  });

  const scenariosQuery = useQuery({
    queryKey: ['projects', selectedProjectId, 'scenarios'],
    queryFn: () => getJSON<Scenario[]>(`/projects/${selectedProjectId}/scenarios`),
    enabled: Boolean(selectedProjectId)
  });

  const runsQuery = useQuery({
    queryKey: ['runs', selectedProjectId],
    queryFn: () => getJSON<RunListItem[]>(`/runs${selectedProjectId ? `?project_id=${selectedProjectId}` : ''}`),
    refetchInterval: location.pathname === '/runs' || location.pathname === '/dashboard' ? 2000 : false
  });

  const mockListenersQuery = useQuery({
    queryKey: ['runtime', 'mock-listeners'],
    queryFn: () => getJSON<MockListenerRuntime[]>('/runtime/mock-listeners'),
    refetchInterval: location.pathname === '/dashboard' || location.pathname === '/mock' ? 2000 : false
  });

  const loadRunsQuery = useQuery({
    queryKey: ['runtime', 'load-runs'],
    queryFn: () => getJSON<LoadRunRuntime[]>('/runtime/load-runs'),
    refetchInterval: location.pathname === '/dashboard' || location.pathname === '/load' ? 2000 : false
  });

  const runDetailQuery = useQuery({
    queryKey: ['run', selectedRunId],
    queryFn: () => getJSON<RunRecord>(`/runs/${selectedRunId}`),
    enabled: Boolean(selectedRunId),
    refetchInterval: location.pathname === '/runs' ? 2000 : false
  });

  const selectedListener = mockListenersQuery.data?.find((item) => item.environment_id === selectedEnvironmentId) ?? null;

  const routesQuery = useQuery({
    queryKey: ['runtime', 'mock-listeners', selectedEnvironmentId, 'routes'],
    queryFn: () => getJSON<RouteStatsItem[]>(`/runtime/mock-listeners/${selectedEnvironmentId}/routes`),
    enabled: Boolean(selectedEnvironmentId && selectedListener),
    refetchInterval: location.pathname === '/dashboard' || location.pathname === '/mock' ? 2000 : false
  });

  const eventsQuery = useQuery({
    queryKey: ['runtime', 'mock-listeners', selectedEnvironmentId, 'events'],
    queryFn: () => getJSON<EventsResponse>(`/runtime/mock-listeners/${selectedEnvironmentId}/events`),
    enabled: Boolean(selectedEnvironmentId && selectedListener),
    refetchInterval: location.pathname === '/mock' ? 2000 : false
  });

  useEffect(() => {
    const firstProjectId = projectsQuery.data?.[0]?.id ?? '';
    if (!selectedProjectId && firstProjectId) {
      setSelectedProjectId(firstProjectId);
    }
  }, [projectsQuery.data, selectedProjectId]);

  useEffect(() => {
    const firstEnvironmentId = environmentsQuery.data?.[0]?.id ?? '';
    if (!selectedEnvironmentId && firstEnvironmentId) {
      setSelectedEnvironmentId(firstEnvironmentId);
      return;
    }
    if (selectedEnvironmentId && !environmentsQuery.data?.some((item) => item.id === selectedEnvironmentId)) {
      setSelectedEnvironmentId(firstEnvironmentId);
    }
  }, [environmentsQuery.data, selectedEnvironmentId]);

  useEffect(() => {
    const firstProfileId = mockProfilesQuery.data?.[0]?.id ?? '';
    if (!selectedMockProfileId && firstProfileId) {
      setSelectedMockProfileId(firstProfileId);
      return;
    }
    if (selectedMockProfileId && !mockProfilesQuery.data?.some((item) => item.id === selectedMockProfileId)) {
      setSelectedMockProfileId(firstProfileId);
    }
  }, [mockProfilesQuery.data, selectedMockProfileId]);

  useEffect(() => {
    const firstProfileId = runProfilesQuery.data?.[0]?.id ?? '';
    if (!selectedRunProfileId && firstProfileId) {
      setSelectedRunProfileId(firstProfileId);
      return;
    }
    if (selectedRunProfileId && !runProfilesQuery.data?.some((item) => item.id === selectedRunProfileId)) {
      setSelectedRunProfileId(firstProfileId);
    }
  }, [runProfilesQuery.data, selectedRunProfileId]);

  useEffect(() => {
    const firstScenarioId = scenariosQuery.data?.[0]?.id ?? '';
    if (!selectedScenarioId && firstScenarioId) {
      setSelectedScenarioId(firstScenarioId);
      return;
    }
    if (selectedScenarioId && !scenariosQuery.data?.some((item) => item.id === selectedScenarioId)) {
      setSelectedScenarioId(firstScenarioId);
    }
  }, [scenariosQuery.data, selectedScenarioId]);

  useEffect(() => {
    const firstRunId = runsQuery.data?.[0]?.id ?? '';
    if (!selectedRunId && firstRunId) {
      setSelectedRunId(firstRunId);
      return;
    }
    if (selectedRunId && !runsQuery.data?.some((item) => item.id === selectedRunId)) {
      setSelectedRunId(firstRunId);
    }
  }, [runsQuery.data, selectedRunId]);

  const selectedProject = projectsQuery.data?.find((item) => item.id === selectedProjectId) ?? null;
  const selectedEnvironment = environmentsQuery.data?.find((item) => item.id === selectedEnvironmentId) ?? null;
  const selectedMockProfile = mockProfilesQuery.data?.find((item) => item.id === selectedMockProfileId) ?? null;
  const selectedRunProfile = runProfilesQuery.data?.find((item) => item.id === selectedRunProfileId) ?? null;
  const selectedScenario = scenariosQuery.data?.find((item) => item.id === selectedScenarioId) ?? null;

  useEffect(() => {
    setEnvironmentDraft(selectedEnvironment ? cloneValue(selectedEnvironment) : null);
  }, [selectedEnvironment]);

  useEffect(() => {
    setMockProfileDraft(selectedMockProfile ? cloneValue(selectedMockProfile) : null);
  }, [selectedMockProfile]);

  useEffect(() => {
    setRunProfileDraft(selectedRunProfile ? cloneValue(selectedRunProfile) : null);
  }, [selectedRunProfile]);

  useEffect(() => {
    setScenarioDraft(selectedScenario ? cloneValue(selectedScenario) : null);
  }, [selectedScenario]);

  const invalidateProject = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['projects'] }),
      queryClient.invalidateQueries({ queryKey: ['projects', selectedProjectId, 'environments'] }),
      queryClient.invalidateQueries({ queryKey: ['projects', selectedProjectId, 'mock-profiles'] }),
      queryClient.invalidateQueries({ queryKey: ['projects', selectedProjectId, 'run-profiles'] }),
      queryClient.invalidateQueries({ queryKey: ['projects', selectedProjectId, 'scenarios'] }),
      queryClient.invalidateQueries({ queryKey: ['runs'] }),
      queryClient.invalidateQueries({ queryKey: ['runtime'] })
    ]);
  };

  const createProject = useMutation({
    mutationFn: () => postJSON<Project>('/projects', { name: projectName, description: projectDescription }),
    onSuccess: async (project) => {
      Toast.success('项目已创建');
      setProjectName('');
      setProjectDescription('');
      setSelectedProjectId(project.id);
      await invalidateProject();
    },
    onError: (error: Error) => Toast.error(error.message)
  });

  const saveEnvironment = useMutation({
    mutationFn: async () => {
      if (!environmentDraft) {
        throw new Error('请先选择或创建一个环境。');
      }
      const payload = cloneValue(environmentDraft);
      if (environmentDraft.id) {
        return putJSON<Environment>(`/environments/${environmentDraft.id}`, payload);
      }
      return postJSON<Environment>(`/projects/${selectedProjectId}/environments`, payload);
    },
    onSuccess: async (environment) => {
      Toast.success('环境已保存');
      if (environment?.id) {
        setSelectedEnvironmentId(environment.id);
      }
      await invalidateProject();
    },
    onError: (error: Error) => Toast.error(error.message)
  });

  const saveMockProfile = useMutation({
    mutationFn: async () => {
      if (!mockProfileDraft) {
        throw new Error('请先选择一个 Mock 配置。');
      }
      return putJSON<MockProfile>(`/mock-profiles/${mockProfileDraft.id}`, {
        name: mockProfileDraft.name,
        config: cloneValue(mockProfileDraft.config)
      });
    },
    onSuccess: async (profile) => {
      Toast.success('Mock 配置已保存');
      if (profile?.id) {
        setSelectedMockProfileId(profile.id);
      }
      await invalidateProject();
    },
    onError: (error: Error) => Toast.error(error.message)
  });

  const saveRunProfile = useMutation({
    mutationFn: async () => {
      if (!runProfileDraft) {
        throw new Error('请先选择一个压测配置。');
      }
      return putJSON<RunProfile>(`/run-profiles/${runProfileDraft.id}`, {
        name: runProfileDraft.name,
        config: cloneValue(runProfileDraft.config)
      });
    },
    onSuccess: async (profile) => {
      Toast.success('压测配置已保存');
      if (profile?.id) {
        setSelectedRunProfileId(profile.id);
      }
      await invalidateProject();
    },
    onError: (error: Error) => Toast.error(error.message)
  });

  const saveScenario = useMutation({
    mutationFn: async () => {
      if (!scenarioDraft) {
        throw new Error('请先选择一个场景。');
      }
      return putJSON<Scenario>(`/scenarios/${scenarioDraft.id}`, {
        name: scenarioDraft.name,
        config: cloneValue(scenarioDraft.config)
      });
    },
    onSuccess: async (scenario) => {
      Toast.success('场景已保存');
      if (scenario?.id) {
        setSelectedScenarioId(scenario.id);
      }
      await invalidateProject();
    },
    onError: (error: Error) => Toast.error(error.message)
  });

  const startMockListener = useMutation({
    mutationFn: () =>
      postJSON('/runtime/mock-listeners', {
        environment_id: selectedEnvironmentId,
        mock_profile_id: selectedMockProfileId
      }),
    onSuccess: async () => {
      Toast.success('Mock 监听器已启动');
      await invalidateProject();
    },
    onError: (error: Error) => Toast.error(error.message)
  });

  const stopMockListener = useMutation({
    mutationFn: () => postJSON(`/runtime/mock-listeners/${selectedEnvironmentId}/stop`, {}),
    onSuccess: async () => {
      Toast.success('Mock 监听器已停止');
      await invalidateProject();
    },
    onError: (error: Error) => Toast.error(error.message)
  });

  const startRun = useMutation({
    mutationFn: () =>
      postJSON('/runs', {
        project_id: selectedProjectId,
        environment_id: selectedEnvironmentId,
        run_profile_id: selectedRunProfileId
      }),
    onSuccess: async () => {
      Toast.success('压测已启动');
      await invalidateProject();
    },
    onError: (error: Error) => Toast.error(error.message)
  });

  const stopRun = useMutation({
    mutationFn: () => postJSON(`/runs/${selectedRunId}/stop`, {}),
    onSuccess: async () => {
      Toast.success('已发送停止请求');
      await invalidateProject();
    },
    onError: (error: Error) => Toast.error(error.message)
  });

  const createEnvironmentDraft = () => {
    setEnvironmentDraft({
      id: '',
      project_id: selectedProjectId,
      name: '新环境',
      target_type: 'internal_mock',
      external_base_url: '',
      default_headers: { 'Content-Type': 'application/json' },
      insecure_skip_verify: false,
      mock_bind_host: '0.0.0.0',
      mock_port: 18881,
      mock_require_auth: false,
      mock_auth_token: 'workbench-token',
      auto_start: true,
      default_mock_profile_id: selectedMockProfileId,
      default_run_profile_id: selectedRunProfileId
    });
  };

  const duplicateMockProfile = async () => {
    if (!selectedProjectId) return;
    const profile = await postJSON<MockProfile>(`/projects/${selectedProjectId}/mock-profiles`, {
      name: `Mock 配置 ${Date.now()}`,
      config: mockProfileDraft ? cloneValue(mockProfileDraft.config) : createDefaultMockProfileConfig()
    });
    setSelectedMockProfileId(profile.id);
    await invalidateProject();
  };

  const duplicateRunProfile = async () => {
    if (!selectedProjectId) return;
    const profile = await postJSON<RunProfile>(`/projects/${selectedProjectId}/run-profiles`, {
      name: `压测配置 ${Date.now()}`,
      config: runProfileDraft ? cloneValue(runProfileDraft.config) : createDefaultRunProfileConfig()
    });
    setSelectedRunProfileId(profile.id);
    await invalidateProject();
  };

  const duplicateScenario = async () => {
    if (!selectedProjectId) return;
    const baseConfig = scenarioDraft ? cloneValue(scenarioDraft.config) : createDefaultScenarioConfig();
    const timestamp = Date.now();
    baseConfig.id = `scenario-${timestamp}`;
    baseConfig.name = `新场景 ${timestamp}`;

    const scenario = await postJSON<Scenario>(`/projects/${selectedProjectId}/scenarios`, {
      name: baseConfig.name,
      config: baseConfig
    });
    setSelectedScenarioId(scenario.id);
    await invalidateProject();
  };

  const deleteEnvironment = async () => {
    if (!environmentDraft?.id) return;
    await deleteJSON(`/environments/${environmentDraft.id}`);
    Toast.success('环境已删除');
    setEnvironmentDraft(null);
    await invalidateProject();
  };

  return {
    forms: {
      projectName,
      setProjectName,
      projectDescription,
      setProjectDescription
    },
    selection: {
      selectedProjectId,
      setSelectedProjectId,
      selectedEnvironmentId,
      setSelectedEnvironmentId,
      selectedMockProfileId,
      setSelectedMockProfileId,
      selectedRunProfileId,
      setSelectedRunProfileId,
      selectedScenarioId,
      setSelectedScenarioId,
      selectedRunId,
      setSelectedRunId
    },
    drafts: {
      environmentDraft,
      setEnvironmentDraft,
      mockProfileDraft,
      setMockProfileDraft,
      runProfileDraft,
      setRunProfileDraft,
      scenarioDraft,
      setScenarioDraft
    },
    selected: {
      selectedProject,
      selectedEnvironment,
      selectedMockProfile,
      selectedRunProfile,
      selectedScenario,
      selectedListener
    },
    queries: {
      projectsQuery,
      environmentsQuery,
      mockProfilesQuery,
      runProfilesQuery,
      scenariosQuery,
      runsQuery,
      mockListenersQuery,
      loadRunsQuery,
      runDetailQuery,
      routesQuery,
      eventsQuery
    },
    mutations: {
      createProject,
      saveEnvironment,
      saveMockProfile,
      saveRunProfile,
      saveScenario,
      startMockListener,
      stopMockListener,
      startRun,
      stopRun
    },
    actions: {
      createEnvironmentDraft,
      duplicateMockProfile,
      duplicateRunProfile,
      duplicateScenario,
      deleteEnvironment,
      invalidateProject
    }
  };
}

type WorkbenchState = ReturnType<typeof useWorkbenchState>;

const WorkbenchContext = createContext<WorkbenchState | null>(null);

export function WorkbenchProvider(props: { children: ReactNode }) {
  const state = useWorkbenchState();
  return <WorkbenchContext.Provider value={state}>{props.children}</WorkbenchContext.Provider>;
}

export function useWorkbench() {
  const context = useContext(WorkbenchContext);
  if (!context) {
    throw new Error('useWorkbench 必须在 WorkbenchProvider 内使用。');
  }
  return context;
}
