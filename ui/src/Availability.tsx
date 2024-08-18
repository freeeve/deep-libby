import {useParams} from "react-router-dom";
import {useState} from "react";
import {AgGridReact} from "ag-grid-react";

import 'ag-grid-community/styles/ag-grid.css';
import 'ag-grid-community/styles/ag-theme-alpine.css';
import {ColDef, SizeColumnsToFitGridStrategy} from "ag-grid-community";

interface SelectedMedia {
    id: string;
    title: string;
    subtitle: string;
    creators: { name: string, role: string }[];
    languages: string[];
    formats: string[];
    description: string;
    coverUrl: string;
    availability: {
        library: { id: string, name: string },
        ownedCount: number,
        availableCount: number,
        holdsCount: number,
        estimatedWaitDays: number
    }[];
}

export default function Availability() {
    //const baseUrl = 'http://localhost:8080/';
    const baseUrl = window.location.origin;
    const {mediaId} = useParams();
    console.log(mediaId);
    const [selectedMedia, setSelectedMedia] = useState<SelectedMedia | null>(null);
    console.log("mediaId", mediaId);
    const columnDefs: ColDef[] = [
        {headerName: 'Library Name', field: 'library.name', minWidth: 500},
        {headerName: 'Owned', field: 'ownedCount', width: 110},
        {headerName: 'Available', field: 'availableCount', sort: 'desc', width: 140},
        {headerName: 'Holds', field: 'holdsCount', width: 110},
        {headerName: 'Estimated Wait Days', field: 'estimatedWaitDays', width: 190},
        {
            headerName: 'Open In Libby',
            field: 'library.id',
            cellRenderer: (params: any) => {
                if (selectedMedia !== null) {
                    return (
                        <a href={`https://libbyapp.com/library/${params.value}/generated-36532/page-1/${selectedMedia.id}`}
                           style={{cursor: 'pointer'}}>
                            open in this library
                        </a>
                    );
                } else {
                    return null; // or some default JSX
                }
            }
        }
    ];

    const clickMedia = (selectedOption: any) => {
        let url = new URL('/api/availability', baseUrl);
        let params: any = {id: selectedOption.id};
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));
        // Fetch the availability data
        fetch(url, {
            method: 'GET',
        })
            .then((response) => response.json())
            .then((data) => {
                // Update the state with the selected book's details and availability data
                setSelectedMedia(data);
            })
            .catch((error) => {
                console.error('Error:', error);
            });
    };
    if (selectedMedia === null) {
        clickMedia({id: mediaId})
    }

    const autoSizeStrategy: SizeColumnsToFitGridStrategy = {
        type: 'fitGridWidth',
    };

    return (
        selectedMedia && (
            <div>
                <h2>Media Availability</h2>
                <div style={{display: 'flex', justifyContent: 'space-between'}}>
                    <div>
                        <div style={{textAlign: 'left'}}>
                            <div><strong>Title:</strong> <strong>{selectedMedia.title}</strong></div>
                            <div>
                                <strong>Creators:</strong> {selectedMedia.creators.map((author) => author.name + ' (' + author.role + ')').join(', ')}
                            </div>
                            <div><strong>Languages:</strong> {selectedMedia.languages.join(', ')}</div>
                            <div><strong>Formats:</strong> {selectedMedia.formats.join(', ')}</div>
                            {selectedMedia.subtitle != "" && (
                                <div><strong>Subtitle:</strong> {selectedMedia.subtitle}</div>)}
                            <div className={'dangerousHTML'}>
                                <div><strong>Description:</strong></div>
                                <div dangerouslySetInnerHTML={{__html: selectedMedia.description}}></div>
                            </div>
                        </div>
                    </div>
                    <img src={selectedMedia.coverUrl}
                         alt={selectedMedia.title}
                         width={0} height={0}
                         sizes="100vw"
                         style={{width: 'auto', height: '100px'}} // optional
                    />
                </div>
                <div className="ag-theme-alpine-auto-dark" style={{height: 600, marginTop: 25}}>
                    <AgGridReact
                        columnDefs={columnDefs}
                        rowData={selectedMedia.availability}
                        defaultColDef={{
                            sortable: true,
                            filter: true,
                            resizable: true
                        }}
                        autoSizeStrategy={autoSizeStrategy}
                    />
                </div>
            </div>
        ));
}