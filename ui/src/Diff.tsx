import {useParams, useNavigate} from "react-router-dom";
import {useEffect, useState} from "react";
import AsyncSelect from 'react-select'
import {AgGridReact} from "ag-grid-react";

import 'ag-grid-community/styles/ag-grid.css';
import 'ag-grid-community/styles/ag-theme-alpine.css';
import {ColDef, SizeColumnsToFitGridStrategy} from "ag-grid-community";

interface DiffResponse {
    diff: SelectedMedia[];
}

interface SelectedMedia {
    id: string;
    title: string;
    subtitle: string;
    creators: { name: string, role: string }[];
    languages: string[];
    formats: string[];
    description: string;
    coverUrl: string;
    library: { id: string, name: string, websiteId: number, isConsortium: boolean };
    availability: {
        library: { id: string, name: string },
        ownedCount: number,
        availableCount: number,
        holdsCount: number,
        estimatedWaitDays: number
    }[];
}

interface Library {
    id: string;
    websiteId: number;
    name: string;
    isConsortium: boolean;
}

interface LibraryOption {
    value: number;
    label: string;
}

export default function Diff() {
    // const baseUrl = 'http://localhost:8080/';
    let baseUrl = window.location.origin;
    if (baseUrl === 'http://localhost:5173') {
        baseUrl = 'http://localhost:8080';
    }
    const navigate = useNavigate();
    const [isFetching, setIsFetching] = useState(false);
    const {leftLibraryId, rightLibraryId} = useParams();
    const leftLibraryIdInt = parseInt(leftLibraryId || '-1');
    const rightLibraryIdInt = parseInt(rightLibraryId || '-1');
    const [libraries, setLibraries] = useState<LibraryOption[]>([]);
    const [filteredRowCount, setFilteredRowCount] = useState(0);
    const [diffResponse, setDiffResponse] = useState<DiffResponse>({diff: []});
    const columnDefs: ColDef[] = [
        {headerName: 'Title', field: 'title', minWidth: 250, sort: 'asc'},
        {
            headerName: 'Creators', field: 'creators',
            valueFormatter: (params) => {
                if (params.value && params.value.length) {
                    return params.value.map((creator: any) => creator.name + ' (' + creator.role + ')').join(', ')
                }
                return '';
            },
        },
        {
            headerName: 'Formats', field: 'formats',
            valueFormatter: (params) => {
                if (params.value && params.value.length) {
                    return params.value.map((format: string) => format).join(', ')
                }
                return '';
            },
        },
        {
            headerName: 'Languages', field: 'languages',
            valueFormatter: (params) => {
                if (params.value && params.value.length) {
                    return params.value.map((language: string) => language).join(', ')
                }
                return '';
            },
        },
        {headerName: 'Owned', field: 'ownedCount', width: 110},
        {headerName: 'Available', field: 'availableCount', width: 140},
        {
            headerName: 'Holds',
            field: 'holdsCount',
            width: 110
        },
        {
            headerName: 'Open In Libby',
            field:
                'library.id',
            cellRenderer:
                (params: any) => {
                    if (diffResponse.diff.length > 0) {
                        return (
                            <a href={`https://libbyapp.com/library/${params.data.library.id}/generated-36532/page-1/${params.data.id}`}
                               style={{cursor: 'pointer'}}>
                                open in left library
                            </a>
                        );
                    } else {
                        return null; // or some default JSX
                    }
                }
        }
    ];

    const libraryOptions = () => {
        let url = new URL('/api/libraries', baseUrl);
        return fetch(url, {
            method: 'GET',
        })
            .then((response) => response.json())
            .then((data) => {
                data.libraries.sort((a: Library, b: Library) => a.name.localeCompare(b.name));
                setLibraries(data.libraries.map((library: Library) => {
                    return {value: library.websiteId, label: library.name};
                }));
            })
            .catch((error) => {
                console.error('Error:', error);
            });
    };

    if (libraries.length === 0) {
        libraryOptions();
    }

    const selectLibraries = (leftId: number, rightId: number) => {
        if (isFetching) {
            return;
        }
        setIsFetching(true);
        let url = new URL('/api/diff', baseUrl);
        let params: any = {leftWebsiteId: leftId, rightWebsiteId: rightId};
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));
        // Fetch the availability data
        fetch(url, {
            method: 'GET',
        })
            .then((response) => response.json())
            .then((data) => {
                // Update the state with the selected book's details and availability data
                setDiffResponse(data);
            })
            .catch((error) => {
                console.error('Error:', error);
            })
            .finally(() => {
                setIsFetching(false);
            });
    };

    const flip = () => {
        navigate('/diff/' + rightLibraryIdInt + '/' + leftLibraryIdInt);
        selectLibraries(rightLibraryIdInt, leftLibraryIdInt);
    }

    const selectLeftLibrary = (selectedOption: any) => {
        navigate('/diff/' + selectedOption.id + '/' + rightLibraryIdInt);
        selectLibraries(selectedOption.id, rightLibraryIdInt);
    }

    const selectRightLibrary = (selectedOption: any) => {
        navigate('/diff/' + leftLibraryIdInt + '/' + selectedOption.id);
        selectLibraries(leftLibraryIdInt, selectedOption.id);
    }

    useEffect(() => {
        if (leftLibraryIdInt !== -1 && rightLibraryIdInt !== -1 && diffResponse.diff.length === 0 && !isFetching) {
            selectLibraries(leftLibraryIdInt, rightLibraryIdInt);
        }
    }, [leftLibraryIdInt, rightLibraryIdInt]);

    const autoSizeStrategy: SizeColumnsToFitGridStrategy = {
        type: 'fitGridWidth',
    };

    return (
        <div>
            <h2>Library Difference</h2>
            <svg width="250" height="180">
                <circle fill="#000" cx="100" cy="90" r="70" stroke="#666" strokeWidth="5"/>
                <circle fill="#242424" cx="160" cy="90" r="70" stroke="#666" strokeWidth="5"/>
            </svg>
            <p>A set difference operation: the library collection on the left side, subtracting the library collection
                on the right side will be in the resulting grid.</p>
            {libraries.length && (
                <div>
                    <div style={{display: 'inline'}}>
                        Left:
                        <AsyncSelect
                            placeholder={"Select left library"}
                            className={"react-select-container"}
                            classNamePrefix={"react-select"}
                            defaultValue={libraries.filter((option: any) => option.value === leftLibraryIdInt)[0]}
                            options={libraries}
                            onChange={(event) => selectLeftLibrary({id: event ? event.value : -1})}>
                        </AsyncSelect>
                    </div>
                    <div style={{display: 'inline'}}>
                        Right:
                        <AsyncSelect
                            placeholder={"Select right library"}
                            className={"react-select-container"}
                            classNamePrefix={"react-select"}
                            defaultValue={libraries.filter((option: any) => option.value === rightLibraryIdInt)[0]}
                            options={libraries}
                            onChange={(event) => selectRightLibrary({id: event ? event.value : -1})}>
                        </AsyncSelect>
                    </div>
                    <a href={""} style={{display: 'block', marginTop: 10}} onClick={() => flip()}>
                        flip left and right
                    </a>
                </div>
            )}
            {diffResponse && (diffResponse.diff.length !== 0) && (
                <div style={{height: 600, marginTop: 25}}>
                    <div>{filteredRowCount} shown out of total {diffResponse.diff.length}</div>
                    <div className="ag-theme-alpine-auto-dark" style={{height: 600, marginTop: 10}}>
                        <AgGridReact
                            columnDefs={columnDefs}
                            rowData={diffResponse.diff}
                            defaultColDef={{
                                sortable: true,
                                filter: true,
                                resizable: true
                            }}
                            autoSizeStrategy={autoSizeStrategy}
                            onFilterChanged={(params) => {
                                setFilteredRowCount(params.api.getDisplayedRowCount());
                            }}
                            onGridReady={(params) => {
                                setFilteredRowCount(params.api.getDisplayedRowCount());
                            }}
                            onRowDataUpdated={(params) => {
                                setFilteredRowCount(params.api.getDisplayedRowCount());
                            }}
                        />
                    </div>
                </div>
            )}
        </div>
    );
}