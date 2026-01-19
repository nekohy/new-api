import React, { useState, useEffect, useMemo, useCallback } from 'react';
import {
  Table,
  Button,
  Space,
  Input,
  Modal,
  Form,
  Typography,
  Tag,
  Popconfirm,
  SideSheet,
  InputNumber,
  Empty,
  Banner,
  Toast,
} from '@douyinfe/semi-ui';
import { IconPlus, IconDelete, IconSetting, IconAlertTriangle } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import useUserGroups from '../hooks/useUserGroups';
import useGroupPrices from '../hooks/useGroupPrices';
import EditableDescription from '../../../../components/common/EditableDescription';

const { Text, Title } = Typography;

const UserGroupSettings = () => {
  const { t } = useTranslation();
  const {
    userGroups,
    loading,
    createUserGroup,
    updateUserGroup,
    deleteUserGroup,
    fetchUserGroupMappings,
    updateUserGroupMappings,
  } = useUserGroups();

  const { groups: modelGroups, fetchGroups } = useGroupPrices(''); // 获取所有模型分组

  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [mappingDrawerVisible, setMappingDrawerVisible] = useState(false);
  const [currentGroup, setCurrentGroup] = useState(null);
  const [mappings, setMappings] = useState([]);
  const [formApi, setFormApi] = useState(null);
  const [groupMappingCounts, setGroupMappingCounts] = useState({}); // 缓存每个用户分组的映射数量
  const [fetchedGroups, setFetchedGroups] = useState(new Set()); // 记录已获取过映射的分组
  const [savingGroup, setSavingGroup] = useState(null); // 记录正在保存描述的分组

  // 组件加载时获取模型分组列表
  useEffect(() => {
    fetchGroups();
  }, [fetchGroups]);

  // 获取所有用户分组的映射数量（防止重复请求）
  useEffect(() => {
    const fetchAllMappingCounts = async () => {
      const counts = {};
      const newFetchedGroups = new Set(fetchedGroups);

      for (const group of userGroups) {
        // 跳过已经获取过的分组
        if (newFetchedGroups.has(group.name)) {
          continue;
        }

        try {
          const data = await fetchUserGroupMappings(group.name);
          counts[group.name] = data.length;
          newFetchedGroups.add(group.name);
        } catch (error) {
          console.error(`Failed to fetch mappings for ${group.name}:`, error);
        }
      }

      if (Object.keys(counts).length > 0) {
        setGroupMappingCounts((prev) => ({ ...prev, ...counts }));
        setFetchedGroups(newFetchedGroups);
      }
    };

    if (userGroups.length > 0) {
      fetchAllMappingCounts();
    }
  }, [userGroups]); // 移除 fetchUserGroupMappings 依赖

  // 打开映射配置抽屉
  const openMappingDrawer = useCallback(async (group) => {
    setCurrentGroup(group);
    const data = await fetchUserGroupMappings(group.name);
    setMappings(data);
    setMappingDrawerVisible(true);
  }, [fetchUserGroupMappings]);

  // 添加模型分组映射
  const addMapping = (modelGroup) => {
    if (mappings.find((m) => m.model_group === modelGroup)) {
      return; // 已存在
    }
    setMappings([...mappings, { model_group: modelGroup, rate_multiplier: 1.0 }]);
  };

  // 删除映射
  const removeMapping = (modelGroup) => {
    setMappings(mappings.filter((m) => m.model_group !== modelGroup));
  };

  // 更新倍率
  const updateMultiplier = (modelGroup, multiplier) => {
    setMappings(
      mappings.map((m) =>
        m.model_group === modelGroup ? { ...m, rate_multiplier: multiplier } : m
      )
    );
  };

  // 保存映射
  const saveMappings = async () => {
    if (!currentGroup) return;
    const success = await updateUserGroupMappings(currentGroup.name, mappings);
    if (success) {
      // 更新映射数量缓存
      setGroupMappingCounts((prev) => ({
        ...prev,
        [currentGroup.name]: mappings.length,
      }));
      // 标记该分组已更新
      setFetchedGroups((prev) => {
        const newSet = new Set(prev);
        newSet.add(currentGroup.name);
        return newSet;
      });
      setMappingDrawerVisible(false);
    }
  };

  // 创建用户分组
  const handleCreate = async (values) => {
    const success = await createUserGroup(values.group_name, values.description);
    if (success) {
      setCreateModalVisible(false);
      formApi && formApi.reset();
      // 新建的分组映射数量为0
      setGroupMappingCounts((prev) => ({
        ...prev,
        [values.group_name]: 0,
      }));
    }
  };

  // 保存描述
  const handleDescriptionSave = useCallback(async (groupName, newDescription) => {
    setSavingGroup(groupName);
    try {
      const success = await updateUserGroup(groupName, newDescription);
      if (!success) {
        throw new Error('更新失败');
      }
    } finally {
      setSavingGroup(null);
    }
  }, [updateUserGroup]);

  const columns = useMemo(() => [
    {
      title: t('分组名称'),
      dataIndex: 'name',
      render: (text) => <Text copyable>{text}</Text>,
    },
    {
      title: t('描述'),
      dataIndex: 'description',
      width: 350,
      render: (text, record) => (
        <EditableDescription
          value={text}
          onSave={(value) => handleDescriptionSave(record.name, value)}
          loading={savingGroup === record.name}
          placeholder={t('点击添加分组描述')}
          maxLength={500}
        />
      ),
    },
    {
      title: t('关联模型分组'),
      key: 'mappings',
      render: (_, record) => {
        const mappingCount = groupMappingCounts[record.name] || 0;
        return (
          <Space>
            <Text>{mappingCount === 0 ? t('未配置') : `${mappingCount} 个`}</Text>
            {mappingCount === 0 && (
              <Tag color="orange" size="small">
                <IconAlertTriangle size="small" style={{ marginRight: 2 }} />
                {t('需要配置')}
              </Tag>
            )}
          </Space>
        );
      },
    },
    {
      title: t('操作'),
      key: 'actions',
      render: (_, record) => (
        <Space>
          <Button
            theme="borderless"
            type="primary"
            icon={<IconSetting />}
            onClick={() => openMappingDrawer(record)}
          >
            {t('配置映射')}
          </Button>
          <Popconfirm
            title={t('确定删除该用户分组吗？')}
            onConfirm={() => deleteUserGroup(record.name)}
          >
            <Button theme="borderless" type="danger" icon={<IconDelete />}>
              {t('删除')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ], [t, savingGroup, groupMappingCounts, handleDescriptionSave, openMappingDrawer, deleteUserGroup]);

  return (
    <div>
      <Banner
        fullMode={false}
        type="info"
        description={t('用户分组用于管理用户和API Key的归属，通过关联模型分组来控制访问权限和定价倍率')}
        style={{ marginBottom: 16 }}
      />

      <Space style={{ marginBottom: 16 }}>
        <Button icon={<IconPlus />} onClick={() => setCreateModalVisible(true)}>
          {t('新建用户分组')}
        </Button>
      </Space>

      <Table
        columns={columns}
        dataSource={userGroups}
        rowKey="name"
        loading={loading}
        pagination={{ pageSize: 20 }}
        empty={<Empty description={t('暂无用户分组，请创建')} />}
      />

      {/* 创建用户分组模态框 */}
      <Modal
        title={t('创建用户分组')}
        visible={createModalVisible}
        onCancel={() => setCreateModalVisible(false)}
        footer={null}
      >
        <Form
          getFormApi={(api) => setFormApi(api)}
          onSubmit={handleCreate}
          labelPosition="left"
          labelWidth={120}
        >
          <Form.Input
            field="group_name"
            label={t('分组名称')}
            rules={[{ required: true, message: t('请输入分组名称') }]}
          />
          <Form.TextArea field="description" label={t('描述')} />
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 16 }}>
            <Space>
              <Button onClick={() => setCreateModalVisible(false)}>{t('取消')}</Button>
              <Button htmlType="submit" type="primary">
                {t('创建')}
              </Button>
            </Space>
          </div>
        </Form>
      </Modal>

      {/* 配置映射侧边栏 */}
      <SideSheet
        title={t('配置模型分组映射') + ` - ${currentGroup?.name || ''}`}
        visible={mappingDrawerVisible}
        onCancel={() => setMappingDrawerVisible(false)}
        width={600}
      >
        <div style={{ padding: 24 }}>
          <Text>{t('选择该用户分组可以访问的模型分组，并设置对应的价格倍率')}</Text>

          <div style={{ marginTop: 16, marginBottom: 16 }}>
            <Text strong>{t('添加模型分组')}</Text>
            <div style={{ marginTop: 8 }}>
              {modelGroups.length === 0 ? (
                <Text type="tertiary">{t('暂无可用的模型分组，请先在模型分组页面创建')}</Text>
              ) : (
                modelGroups.map((mg) => (
                  <Button
                    key={mg}
                    size="small"
                    style={{ marginRight: 8, marginBottom: 8 }}
                    onClick={() => addMapping(mg)}
                    disabled={mappings.find((m) => m.model_group === mg)}
                  >
                    {mg}
                  </Button>
                ))
              )}
            </div>
          </div>

          <div style={{ marginTop: 24 }}>
            <Text strong>{t('已配置映射')}</Text>
            {mappings.length === 0 ? (
              <Empty
                description={t('未配置任何模型分组，用户将无法使用任何模型')}
                style={{ marginTop: 16 }}
              />
            ) : (
              <Table
                dataSource={mappings}
                rowKey="model_group"
                pagination={false}
                style={{ marginTop: 8 }}
                columns={[
                  {
                    title: t('模型分组'),
                    dataIndex: 'model_group',
                  },
                  {
                    title: t('价格倍率'),
                    dataIndex: 'rate_multiplier',
                    render: (val, record) => (
                      <InputNumber
                        value={val}
                        min={0.01}
                        max={100}
                        step={0.1}
                        prefix="×"
                        style={{ width: 120 }}
                        onChange={(v) => updateMultiplier(record.model_group, v)}
                      />
                    ),
                  },
                  {
                    title: t('操作'),
                    key: 'actions',
                    render: (_, record) => (
                      <Button
                        theme="borderless"
                        type="danger"
                        size="small"
                        icon={<IconDelete />}
                        onClick={() => removeMapping(record.model_group)}
                      >
                        {t('删除')}
                      </Button>
                    ),
                  },
                ]}
              />
            )}
          </div>

          <div style={{ marginTop: 24, display: 'flex', justifyContent: 'flex-end' }}>
            <Space>
              <Button onClick={() => setMappingDrawerVisible(false)}>{t('取消')}</Button>
              <Button type="primary" onClick={saveMappings}>
                {t('保存')}
              </Button>
            </Space>
          </div>
        </div>
      </SideSheet>
    </div>
  );
};

export default UserGroupSettings;
