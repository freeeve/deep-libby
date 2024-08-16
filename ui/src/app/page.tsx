'use client';

import Image from "next/image";
import styles from "./page.module.css";
import {useEffect, useState} from "react";
import Select from "react-select/base";
import SearchWindow from "./search-window";
import AsyncSelect from "react-select/async";

export default function Home() {

    const [data, setData] = useState({results: []});
    let selectedOption = null;
    const [selectedBook, setSelectedBook] = useState(null);

    useEffect(() => {
        //let url = new URL('/api/search', window.location.origin);
        let url = new URL('/api/search', 'http://localhost:8080/');
        let params = {q: 'covenant'};
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));

        fetch(url, {
            method: 'GET',
        })
            .then(response => response.json())
            .then(data => setData(data))
            .catch((error) => {
                console.error('Error:', error);
            });
    }, []);

    const loadOptions = (inputValue, callback) => {
        let url = new URL('/api/search', 'http://localhost:8080/');
        let params = {q: inputValue};
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));

        fetch(url, {
            method: 'GET',
        })
            .then(response => response.json())
            .then(data => callback(data.results))
            .catch((error) => {
                console.error('Error:', error);
            });
    }


    const formatOptionLabel = ({title, creators, formats, coverUrl}) => (
        <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <div>
                <div><strong>{title}</strong></div>
                {creators.map((creator, index) => (
                    <div key={index}>{creator.name} ({creator.role})</div>
                ))}
                <div>{formats.join(', ')}</div>
            </div>
            <img src={coverUrl} alt={title} style={{height: 50, marginLeft: 10}}/>
        </div>
    );

    const customStyles = {
        control: (provided) => ({
            ...provided,
            width: 800, // You can adjust this value as needed
            backgroundColor: 'rgba(255, 255, 255, 0.1)', // Adjust this as needed
        }),
        option: (provided, state) => ({
            ...provided,
            backgroundColor: 'rgba(0 , 0, 0, 0.8)',
            color: 'white',
        }),
        singleValue: (provided) => ({
            ...provided,
            color: 'white',
        }),
    };

    const handleChange = (selectedOption) => {
        // Fetch the availability data
        fetch(`http://localhost:8080/api/availability?id=${selectedOption.id}`)
            .then((response) => response.json())
            .then((data) => {
                // Update the state with the selected book's details and availability data
                setSelectedBook({ ...selectedOption, availability: data.availability });
            })
            .catch((error) => {
                console.error('Error:', error);
            });
    };


    return (
        <main className={styles.main}>
            <div>
                <AsyncSelect
                    loadOptions={loadOptions}
                    formatOptionLabel={formatOptionLabel}
                    styles={customStyles}
                    components={{
                        MenuList: SearchWindow
                    }}
                    onChange={handleChange} // Use the handleChange function here
                />
                {selectedBook && (
                    <div>
                        {/* Display the selected book's details and availability data */}
                        <h2>{selectedBook.title}</h2>
                        <Image src={selectedBook.coverUrl}
                               alt={selectedBook.title}
                               width={0} height={0}
                               sizes="100vw"
                               style={{ width: 'auto', height: '100px' }} // optional
                        />
                        <p>{selectedBook.description}</p>
                        {/* ...other details... */}
                        {selectedBook.availability.map((availability, index) => (
                            <div key={index}>
                                <h3>{availability.library.name}</h3>
                                <p>Owned: {availability.ownedCount}</p>
                                <p>Available: {availability.availableCount}</p>
                                <p>Holds: {availability.holdsCount}</p>
                                <p>Estimated Wait Days: {availability.estimatedWaitDays}</p>
                            </div>
                        ))}
                    </div>
                )}
            </div>
        </main>
    );
}
