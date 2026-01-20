import type { Sidebar } from 'vocs';

export const sidebar: Sidebar =
    [
        {
            text: 'Introduction',
            items: [
                {
                    text: 'What is OpenSigner?',
                    link: '/introduction/about',
                },
                {
                    text: 'Users',
                    link: '/introduction/users',
                },
                {
                    text: 'Setup',
                    link: '/introduction/setup',
                },
                {
                    text: 'Getting Started',
                    link: '/introduction/getting-started',
                },
            ]
        },
        {
            text: 'Security',
            items: [
                {
                    text: 'Overview',
                    link: '/security/overview',
                },
                {
                    text: 'Recovery Methods',
                    link: '/security/recovery-methods',
                },
                {
                    text: 'Deployment Scenarios',
                    link: '/security/deployment-scenarios',
                },
                {
                    text: 'Threat Analysis',
                    link: '/security/threat-analysis',
                },
                {
                    text: 'System Integrity',
                    link: '/security/system-integrity',
                },
            ]
        },
        {
            text: 'Actions',
            items: [
                {
                    text: 'Creating a Key',
                    link: '/actions/signup',
                },
                {
                    text: 'Recovering a Key',
                    link: '/actions/login',
                },
                {
                    text: 'Using a Key',
                    link: '/actions/operation',
                },
            ]
        },
        {
            text: 'Components',
            items: [
                {
                    text: 'Authentication',
                    link: '/components/auth',
                },
                {
                    text: 'iFrame',
                    link: '/components/iframe',
                },
                {
                    text: 'Hot Storage',
                    link: '/components/hot_storage',
                },
                {
                    text: 'Cold Storage',
                    link: '/components/shield',
                },
            ]
        },
        {
            text: "APIs",
            items: [
                {
                    text: 'Postman Collection',
                    link: '/apis/postman',
                },
                {
                    text: 'Authentication Service',
                    link: '/apis/auth_service',
                },
                {
                    text: 'Hot Storage',
                    link: '/apis/hot_storage',
                },
                {
                    text: 'Cold Storage',
                    link: '/apis/cold_storage',
                },
            ]
        },
    ];
