import {useParams, useNavigate} from "react-router-dom";
import {useEffect, useState} from "react";
import {AgGridReact} from "ag-grid-react";

import 'ag-grid-community/styles/ag-grid.css';
import 'ag-grid-community/styles/ag-theme-alpine.css';
import {ColDef, SizeColumnsToFitGridStrategy} from "ag-grid-community";
import AsyncSelect from "react-select";
import SearchMedia from "./SearchMedia.tsx";

interface IntersectResponse {
    intersect: SelectedMedia[];
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
    leftLibraryMediaCounts: {
        library: { id: string, name: string, websiteId: number, isConsortium: boolean }
        ownedCount: number,
        availableCount: number,
        holdsCount: number,
        estimatedWaitDays: number
    };
    rightLibraryMediaCounts: {
        library: { id: string, name: string, websiteId: number, isConsortium: boolean }
        ownedCount: number,
        availableCount: number,
        holdsCount: number,
        estimatedWaitDays: number
    };
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

export default function Intersect() {
    // const baseUrl = 'http://localhost:8080/';
    let baseUrl = window.location.origin;
    if (baseUrl === 'http://localhost:5173') {
        baseUrl = 'http://localhost:8080';
    }
    const [isFetching, setIsFetching] = useState(false);
    const navigate = useNavigate();
    const [filteredRowCount, setFilteredRowCount] = useState(0);
    let {leftLibraryId = '', rightLibraryId = ''} = useParams();
    const [libraries, setLibraries] = useState<LibraryOption[]>([]);
    const [intersectResponse, setIntersectResponse] = useState<IntersectResponse>({intersect: []});
    const columnDefs: ColDef[] = [
        {headerName: 'Title', field: 'title', minWidth: 250, sort: 'asc'},
        {
            headerName: 'Creator Names', field: 'creators',
            valueFormatter: (params: any) => {
                if (params.value && params.value.length) {
                    return params.value.map((creator: any) => creator.name).join(', ');
                }
                return '';
            }
        },
        {
            headerName: 'Formats', field: 'formats',
            valueFormatter: (params: any) => {
                if (params.value && params.value.length) {
                    return params.value.join(', ');
                }
                return '';
            }
        },

        {
            headerName: 'Languages', field: 'languages',
            valueFormatter: (params: any) => {
                if (params.value && params.value.length) {
                    return params.value.join(', ');
                }
                return '';
            }
        },
        {
            headerName: 'Left Counts (owned/available/holds/estimated wait days)',
            field: 'leftLibraryMediaCounts.ownedCount',
            width: 150,
            valueFormatter: (params: any) => {
                return params.data.leftLibraryMediaCounts.ownedCount + ' / ' +
                    params.data.leftLibraryMediaCounts.availableCount + ' / ' +
                    params.data.leftLibraryMediaCounts.holdsCount + ' / ' +
                    params.data.leftLibraryMediaCounts.estimatedWaitDays;
            }
        },
        {
            headerName: 'Open In Libby',
            field: 'library.id',
            cellRenderer: (params: any) => {
                if (intersectResponse.intersect.length > 0) {
                    return (
                        <a href={`https://libbyapp.com/library/${params.data.leftLibraryMediaCounts.library.id}/generated-36532/page-1/${params.data.id}`}
                           style={{cursor: 'pointer'}}>
                            open in left library
                        </a>
                    );
                } else {
                    return null; // or some default JSX
                }
            }
        },
        {
            headerName: 'Right Counts (owned/available/holds/estimated wait days)',
            field: 'rightLibraryMediaCounts.ownedCount',
            width: 150,
            valueFormatter: (params: any) => {
                return params.data.rightLibraryMediaCounts.ownedCount + ' / ' +
                    params.data.rightLibraryMediaCounts.availableCount + ' / ' +
                    params.data.rightLibraryMediaCounts.holdsCount + ' / ' +
                    params.data.rightLibraryMediaCounts.estimatedWaitDays;
            }
        },
        {
            headerName: 'Open In Libby',
            field: 'library.id',
            cellRenderer: (params: any) => {
                if (intersectResponse.intersect.length > 0) {
                    return (
                        <a href={`https://libbyapp.com/library/${params.data.rightLibraryMediaCounts.library.id}/generated-36532/page-1/${params.data.id}`}
                           style={{cursor: 'pointer'}}>
                            open in right library
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
                    return {value: library.id, label: library.name};
                }));
            })
            .catch((error) => {
                console.error('Error:', error);
            });
    };

    if (libraries.length === 0) {
        libraryOptions();
    }

    const selectLibraries = (leftId: string, rightId: string) => {
        if (isFetching || leftId == '' || rightId == '') {
            return;
        }
        setIsFetching(true);
        let url = new URL('/api/intersect', baseUrl);
        let params: any = {leftLibraryId: leftId, rightLibraryId: rightId};
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));
        // Fetch the availability data
        fetch(url, {
            method: 'GET',
        })
            .then((response) => response.json())
            .then((data) => {
                // Update the state with the selected book's details and availability data
                setIntersectResponse(data);
            })
            .catch((error) => {
                console.error('Error:', error);
            })
            .finally(() => {
                setIsFetching(false); // Add this line
            });
    };

    const selectLeftLibrary = (selectedOption: any) => {
        navigate('/intersect/' + selectedOption.id + '/' + leftLibraryId);
        selectLibraries(selectedOption.id, rightLibraryId);
    }

    const selectRightLibrary = (selectedOption: any) => {
        navigate('/intersect/' + leftLibraryId + '/' + selectedOption.id);
        selectLibraries(leftLibraryId, selectedOption.id);
    }

    useEffect(() => {
        if (leftLibraryId !== '' && rightLibraryId !== '' && !isFetching) {
            selectLibraries(leftLibraryId, rightLibraryId);
        }
    }, [leftLibraryId, rightLibraryId]);

    const autoSizeStrategy: SizeColumnsToFitGridStrategy = {
        type: 'fitGridWidth',
    };

    return (
        <div>
            <SearchMedia></SearchMedia>
            <h2>Library Intersection</h2>
            <svg width="250" height="180" xmlns="http://www.w3.org/2000/svg">
                <defs>
                    <clipPath id="mask_left">
                        <circle r="70" id="circle_right" cy="90" cx="160" strokeWidth="5" stroke="#666"
                                fill="none"/>
                    </clipPath>
                </defs>
                <g>
                    <circle r="70" id="center" cy="90" cx="100" fill="#000" clipPath="url(#mask_left)"/>
                    <circle r="70" id="circle_left" cy="90" cx="100" strokeWidth="5" stroke="#666" fill="none"/>
                    <circle r="70" id="circle_right" cy="90" cx="160" strokeWidth="5" stroke="#666" fill="none"/>
                </g>
            </svg>
            <p>A set intersection operation, the library collection on the left side, intersecting the library
                collection
                on the right side will be in the resulting grid.</p>
            <div>
                {libraries.length && (
                    <div>
                        <div style={{display: 'inline'}}>
                            <AsyncSelect
                                placeholder={"Select left library"}
                                className={"react-select-container"}
                                classNamePrefix={"react-select"}
                                defaultValue={libraries.filter((option: any) => option.value === leftLibraryId)[0]}
                                options={libraries}
                                onChange={(event) => selectLeftLibrary({id: event ? event.value : ''})}>
                            </AsyncSelect>
                        </div>
                        <div style={{display: 'inline'}}>
                            <AsyncSelect
                                placeholder={"Select right library"}
                                className={"react-select-container"}
                                classNamePrefix={"react-select"}
                                defaultValue={libraries.filter((option: any) => option.value === rightLibraryId)[0]}
                                options={libraries}
                                onChange={(event) => selectRightLibrary({id: event ? event.value : ''})}>
                            </AsyncSelect>
                        </div>
                    </div>
                )}
            </div>
            <div>
                {intersectResponse.intersect.length && (
                    <div style={{marginTop: 25}}>
                        <div>{filteredRowCount} shown out of total {intersectResponse.intersect.length}</div>
                        <div className="ag-theme-alpine-auto-dark" style={{height: 600, marginTop: 10}}>
                            <AgGridReact
                                columnDefs={columnDefs}
                                rowData={intersectResponse.intersect}
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
        </div>
    );
}